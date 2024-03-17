package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"

	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	nameHashLength               = 10
	proxyPodContainerName        = "proxy"
	proxyPodImage                = "openresty/openresty:1.21.4.1-0-jammy"
	proxyPodImagePullPolicy      = "IfNotPresent"
	proxyResourceNameBase        = "kubectl-portal-proxy"
	proxyVolumeName              = "proxy-volume"
	proxyClusterDomainEnv        = "KUBECTL_PORTAL_CLUSTER_DOMAIN"
	defaultPort             uint = 7070
	defaultClusterDomain         = "cluster.local"
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
	ContainerPort int `json:"containerPort"`
}

type Container struct {
	Name            string      `json:"name"`
	Image           string      `json:"image"`
	ImagePullPolicy string      `json:"imagePullPolicy"`
	Ports           []Port      `json:"ports"`
	VolumeMounts    []stringMap `json:"volumeMounts"`
	Env             []stringMap `json:"env"`
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
	clusterDomain     string

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
			Name:            proxyPodContainerName,
			Image:           kp.image,
			ImagePullPolicy: kp.pullPolicy,
			Ports:           []Port{{ContainerPort: 80}},
			VolumeMounts: []stringMap{
				{
					"mountPath": "/etc/nginx/conf.d/default.conf",
					"name":      proxyVolumeName,
					"subPath":   "default.conf",
				},
				{
					"mountPath": "/app/access.lua",
					"name":      proxyVolumeName,
					"subPath":   "access.lua",
				},
				{
					"mountPath": "/usr/local/openresty/nginx/conf/nginx.conf",
					"name":      proxyVolumeName,
					"subPath":   "nginx.conf",
				},
			},
			Env: []stringMap{
				{"name": proxyClusterDomainEnv, "value": kp.clusterDomain},
			},
		},
	}
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
	config.Data["access.lua"] = readEmbeddedFile("data/access.lua")
	config.Data["default.conf"] = readEmbeddedFile("data/default.conf")
	config.Data["nginx.conf"] = readEmbeddedFile("data/nginx.conf")
	return config
}

func (kp *kubectlPortal) deleteProxyResources() error {
	kp.vprintf("deleting proxy resources...\n")

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
	return nil
}

func (kp *kubectlPortal) createProxyResources() error {
	kp.printf("creating proxy resources...\n")

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

	kc := newKubectl("apply", "-f", "-").namespace(kp.namespace)
	out, err := kc.run(buf.Bytes())
	if err != nil {
		return fmt.Errorf("'kubectl apply' failed: %w\n%v", err, string(out))
	}
	return nil
}

func (kp *kubectlPortal) waitForProxyPod() error {
	kp.printf("waiting for proxy to be ready...\n")

	kc := newKubectl(
		"wait",
		"--for=condition=Ready",
		"pod/"+proxyResourceName(),
	).namespace(kp.namespace)

	out, err := kc.run(nil)
	if err != nil {
		return fmt.Errorf("'kubectl wait' failed: %w\n%s", err, string(out))
	}
	return err
}

func (kp *kubectlPortal) portForwardProxyPod() error {
	kc := newKubectl(
		"port-forward",
		proxyResourceName(),
		fmt.Sprintf("%v:80", kp.port),
	).namespace(kp.namespace)

	cmd, err := kc.start()
	if err != nil {
		return fmt.Errorf("'kubectl port-forward' failed: %w\n", err)
	}

	kp.printf("kubectl port-forward now running at localhost:%v\n", kp.port)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	fmt.Println("\ninterrupt received")

	err = cmd.Process.Signal(os.Interrupt)
	if err != nil {
		return fmt.Errorf("interrupt 'kubectl port-forward' failed: %w\n", err)
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("wait 'kubectl port-forward' failed: %w\n", err)
	}

	return nil
}

func (kp *kubectlPortal) run() error {
	err := kp.deleteProxyResources()
	if err != nil {
		return err
	}

	err = kp.createProxyResources()
	if err != nil {
		return err
	}
	defer func() {
		err2 := kp.deleteProxyResources()
		// TODO: Group errors
		if err == nil {
			err = err2
		}
	}()

	err = kp.waitForProxyPod()
	if err != nil {
		return err
	}
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

	configFlags := genericclioptions.ConfigFlags{
		Namespace: &kp.namespace,
	}
	configFlags.AddFlags(flags)
	flags.BoolVarP(&help, "help", "h", false, "Show usage help")
	flags.BoolVar(&kp.verbose, "portal-verbose", false, "Enable verbose mode for kubectl-portal")
	flags.UintVar(&kp.port, "portal-port", defaultPort, "Local port to use for HTTP proxy")
	flags.StringVar(&kp.image, "portal-image", proxyPodImage, "Image to use for HTTP proxy")
	flags.StringVar(&kp.proxyResourceName, "portal-name", defaultResourceName, "Pod/ConfigMap name to use for HTTP proxy")
	flags.StringVar(&kp.pullPolicy, "portal-pull-policy", proxyPodImagePullPolicy, "Image pull policy to use for HTTP proxy")
	flags.StringVar(&kp.clusterDomain, "portal-cluster-domain", defaultClusterDomain, "Cluster domain to use in HTTP proxy DNS resolution")

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
