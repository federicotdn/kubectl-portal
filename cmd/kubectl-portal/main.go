package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strings"

	"github.com/spf13/pflag"
)

const (
	nameHashLength               = 10
	proxyPodImage                = "golang:1.22.1"
	proxyPodImagePullPolicy      = "IfNotPresent"
	proxyResourceNameBase        = "kubectl-portal-proxy"
	proxyVolumeName              = "proxy-volume"
	defaultPort             uint = 7070
	internalPort            uint = 8080
)

//go:embed data
var data embed.FS

type stringMap map[string]string

type Resource struct {
	ApiVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

type Port struct {
	ContainerPort uint `json:"containerPort"`
}

type ReadinessProbe struct {
	TcpSocket struct {
		Port uint `json:"port"`
	} `json:"tcpSocket"`
	PeriodSeconds int `json:"periodSeconds"`
}

type Container struct {
	Name            string         `json:"name"`
	Image           string         `json:"image"`
	ImagePullPolicy string         `json:"imagePullPolicy"`
	Ports           []Port         `json:"ports"`
	VolumeMounts    []stringMap    `json:"volumeMounts"`
	Command         []string       `json:"command,omitempty"`
	Args            []string       `json:"args,omitempty"`
	ReadinessProbe  ReadinessProbe `json:"readinessProbe"`
}

type Volume struct {
	Name      string    `json:"name"`
	ConfigMap stringMap `json:"configMap"`
}

type Pod struct {
	Resource
	Spec struct {
		Containers []Container `json:"containers"`
		Volumes    []Volume    `json:"volumes"`
	} `json:"spec"`
}

type ConfigMap struct {
	Resource
	Data stringMap `json:"data"`
}

type kubectlPortal struct {
	proxyResourceName string
	image             string
	pullPolicy        string
	port              uint

	namespace string
	verbose   bool
}

type kubectlCmd struct {
	args []string
}

func readEmbeddedFile(fileName string) string {
	data, err := data.ReadFile(fileName)
	if err != nil {
		panicf("error: unable to read embedded file: %v", err)
	}
	return string(data)
}

func newKubectl(args ...string) *kubectlCmd {
	return &kubectlCmd{args: args}
}

func (kc *kubectlCmd) namespace(namespace string) *kubectlCmd {
	if namespace != "" {
		kc.args = append([]string{"--namespace", namespace}, kc.args...)
	}
	return kc
}

func (kc *kubectlCmd) run(input []byte) ([]byte, error) {
	var outbuf, outerr bytes.Buffer
	cmd := exec.Command("kubectl", kc.args...)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	cmd.Stdout = &outbuf
	cmd.Stderr = &outerr
	err := cmd.Run()
	if err != nil {
		return outerr.Bytes(), err
	}

	return outbuf.Bytes(), nil
}

func (kc *kubectlCmd) start() (*exec.Cmd, error) {
	cmd := exec.Command("kubectl", kc.args...)
	cmd.Stderr = os.Stderr
	return cmd, cmd.Start()
}

func proxyResourceName() string {
	hostname, err := os.Hostname()
	if err != nil {
		panicf("error: unable to retrieve hostname: %v", err)
	}
	user, err := user.Current()
	if err != nil {
		panicf("error: unable to retrieve user: %v", err)
	}

	h := sha256.New()
	h.Write([]byte(user.Name + "@" + hostname))
	hash := hex.EncodeToString(h.Sum(nil))
	return proxyResourceNameBase + "-" + hash[:nameHashLength]
}

func (kp *kubectlPortal) proxyPod() Pod {
	pod := Pod{
		Resource: Resource{ApiVersion: "v1", Kind: "Pod"},
	}
	pod.Resource.Metadata.Name = kp.proxyResourceName
	pod.Spec.Containers = []Container{
		{
			Name:            "proxy",
			Image:           kp.image,
			ImagePullPolicy: kp.pullPolicy,
			Ports:           []Port{{ContainerPort: internalPort}},
			VolumeMounts: []stringMap{
				{
					"mountPath": "/app/go.mod",
					"name":      proxyVolumeName,
					"subPath":   "go.mod",
				},
				{
					"mountPath": "/app/go.sum",
					"name":      proxyVolumeName,
					"subPath":   "go.sum",
				},
				{
					"mountPath": "/app/main.go",
					"name":      proxyVolumeName,
					"subPath":   "main.go",
				},
			},
			// Compile and run kubectl-portal-proxy. The service is
			// tiny and has almost no dependencies, so this is not
			// expensive. It also allows for not creating a custom
			// Docker image for kubectl-portal-proxy.
			Command: []string{"/bin/bash"},
			Args:    []string{"-c", "cd /app && go run main.go"},
		},
	}
	pod.Spec.Containers[0].ReadinessProbe.TcpSocket.Port = internalPort
	pod.Spec.Containers[0].ReadinessProbe.PeriodSeconds = 1
	pod.Spec.Volumes = []Volume{
		{
			Name:      proxyVolumeName,
			ConfigMap: stringMap{"name": kp.proxyResourceName},
		},
	}
	return pod
}

func (kp *kubectlPortal) proxyConfigMap() ConfigMap {
	config := ConfigMap{
		Resource: Resource{ApiVersion: "v1", Kind: "ConfigMap"},
		Data:     make(stringMap),
	}
	config.Resource.Metadata.Name = kp.proxyResourceName
	config.Data["go.mod"] = readEmbeddedFile("data/go.mod.copy")
	config.Data["go.sum"] = readEmbeddedFile("data/go.sum.copy")
	config.Data["main.go"] = readEmbeddedFile("data/main.go")
	return config
}

func (kp *kubectlPortal) deleteProxyResources() error {
	kp.vprintf("Deleting proxy resources...\n")

	kc := newKubectl(
		"delete",
		"pod,configmap",
		proxyResourceName(),
		"--ignore-not-found",
	).namespace(kp.namespace)

	out, err := kc.run(nil)
	if err != nil {
		return fmt.Errorf("'kubectl delete' failed: %w\n%v", err, string(out))
	}

	kp.vprintf("Resources deleted\n")
	return nil
}

func (kp *kubectlPortal) createProxyResources() (string, error) {
	kp.printf("Creating proxy resources...\n")

	buf := bytes.Buffer{}
	enc := json.NewEncoder(&buf)

	err := enc.Encode(kp.proxyPod())
	if err != nil {
		panicf("error: unable to marshal Pod data: %s", err)
	}

	buf.WriteString("\n")

	err = enc.Encode(kp.proxyConfigMap())
	if err != nil {
		panicf("error: unable to marshal ConfigMap data: %s", err)
	}

	kc := newKubectl(
		"apply",
		"-f",
		"-",
		"-o",
		"jsonpath={.items[0].metadata.namespace}",
	).namespace(kp.namespace)

	out, err := kc.run(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("'kubectl apply' failed: %w\n%v", err, string(out))
	}

	kp.printf("Resources created\n")
	return strings.TrimSpace(string(out)), nil
}

func (kp *kubectlPortal) waitForProxyPod() error {
	kp.printf("Waiting for proxy to be ready...\n")

	kc := newKubectl(
		"wait",
		"--for=condition=Ready",
		"pod/"+proxyResourceName(),
	).namespace(kp.namespace)

	out, err := kc.run(nil)
	if err != nil {
		return fmt.Errorf("'kubectl wait' failed: %w\n%s", err, string(out))
	}

	return nil
}

func (kp *kubectlPortal) portForwardProxyPod() error {
	kp.printf("Listening at localhost:%v\n", kp.port)

	kc := newKubectl(
		"port-forward",
		proxyResourceName(),
		fmt.Sprintf("%v:%v", kp.port, internalPort),
	).namespace(kp.namespace)

	cmd, err := kc.start()
	if err != nil {
		return fmt.Errorf("'kubectl port-forward' failed: %w\n", err)
	}

	interrupt := make(chan os.Signal, 1)
	done := make(chan error, 1)

	go func() {
		done <- cmd.Wait()
	}()

	signal.Notify(interrupt, os.Interrupt)

	for {
		select {
		case <-interrupt:
			kp.printf("Interrupt signal received\n")

			err = cmd.Process.Signal(os.Interrupt)
			if err != nil {
				return fmt.Errorf("interrupt 'kubectl port-forward' failed: %w\n", err)
			}
		case err = <-done:
			if err != nil {
				return fmt.Errorf("wait 'kubectl port-forward' failed: %w\n", err)
			}
			return nil
		}
	}
}

func (kp *kubectlPortal) run() error {
	err := kp.deleteProxyResources()
	if err != nil {
		return err
	}

	effectiveNs, err := kp.createProxyResources()
	if err != nil {
		return err
	}
	defer func() {
		kp.printf("Cleaning up...\n")

		deleteErr := kp.deleteProxyResources()
		err = errors.Join(err, deleteErr)
	}()

	err = kp.waitForProxyPod()
	if err != nil {
		return err
	}

	kp.printf("Proxy is ready (namespace: %v)\n", effectiveNs)

	err = kp.portForwardProxyPod()
	return err
}

func panicf(format string, a ...any) {
	panic(fmt.Sprintf(format, a...))
}

func (kp *kubectlPortal) eprintf(err error) {
	fmt.Fprintf(os.Stderr, "error: %v", err)
	os.Exit(1)
}

func (kp *kubectlPortal) printf(format string, a ...any) {
	fmt.Printf(format, a...)
}

func (kp *kubectlPortal) vprintf(format string, a ...any) {
	if kp.verbose {
		kp.printf(format, a...)
	}
}

func parseFlags(kp *kubectlPortal) error {
	help := false
	defaultResourceName := proxyResourceName()

	flags := pflag.NewFlagSet("kubectl-portal", pflag.ContinueOnError)
	pflag.CommandLine = flags

	// kubectl flags
	// Taken from:
	// https://github.com/kubernetes/cli-runtime/blob/master/pkg/genericclioptions/config_flags.go
	flags.StringVarP(&kp.namespace, "namespace", "n", "", "If present, the namespace scope for this CLI request")

	// Custom flags
	flags.BoolVarP(&help, "help", "h", false, "Show usage help")
	flags.BoolVar(&kp.verbose, "portal-verbose", false, "Enable verbose mode for kubectl-portal")
	flags.UintVar(&kp.port, "portal-port", defaultPort, "Local port to use for HTTP proxy")
	flags.StringVar(&kp.image, "portal-image", proxyPodImage, "Image to use for HTTP proxy")
	flags.StringVar(&kp.proxyResourceName, "portal-name", defaultResourceName, "Pod/ConfigMap name to use for proxy")
	flags.StringVar(&kp.pullPolicy, "portal-pull-policy", proxyPodImagePullPolicy, "Image pull policy to use for proxy")

	err := flags.Parse(os.Args)
	if err != nil {
		return fmt.Errorf("%w\nSee 'kubectl portal --help' for usage.\n", err)
	}

	if help {
		fmt.Printf("Options:\n%v", flags.FlagUsages())
		os.Exit(0)
	}

	return nil
}

func main() {
	kp := &kubectlPortal{}
	err := parseFlags(kp)
	if err != nil {
		kp.eprintf(err)
	}

	err = kp.run()
	if err != nil {
		kp.eprintf(err)
	}
}
