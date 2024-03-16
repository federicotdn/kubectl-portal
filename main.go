package main

import (
	"bytes"
	"crypto/sha256"
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
	proxyPodContainerName        = "nginx"
	proxyPodImageVersion         = "1.0.0"
	proxyPodImageBase            = "federicotedin/kubectl-portal-nginx"
	proxyPodImagePullPolicy      = "IfNotPresent"
	proxyPodNameBase             = "kubectl-portal-nginx"
	defaultPort             uint = 7070
)

type stringMap map[string]string

type Port struct {
	ContainerPort int `json:"containerPort"`
}

type Container struct {
	Name            string `json:"name"`
	Image           string `json:"image"`
	ImagePullPolicy string `json:"imagePullPolicy"`
	Ports           []Port `json:"ports"`
}

type Pod struct {
	ApiVersion string    `json:"apiVersion"`
	Kind       string    `json:"kind"`
	Metadata   stringMap `json:"metadata"`
	Spec       struct {
		Containers []Container `json:"containers"`
	} `json:"spec"`
}

type kubectlPortal struct {
	proxyPodName string

	namespace  string
	image      string
	pullPolicy string
	port       uint
}

type kubectlCmd struct {
	args []string
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

func proxyPodName() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(fmt.Sprintf("eror: unable to retrieve hostname: %v", err))
	}
	user, err := user.Current()
	if err != nil {
		panic(fmt.Sprintf("eror: unable to retrieve user: %v", err))
	}

	h := sha256.New()
	h.Write([]byte(user.Name + "@" + hostname))
	hash := hex.EncodeToString(h.Sum(nil))
	return proxyPodNameBase + "-" + hash[:nameHashLength]
}

func (kp *kubectlPortal) proxyPod() Pod {
	pod := Pod{
		ApiVersion: "v1",
		Kind:       "Pod",
		Metadata: stringMap{
			"name": kp.proxyPodName,
		},
	}
	pod.Spec.Containers = []Container{
		{
			Name:            proxyPodContainerName,
			Image:           kp.image,
			ImagePullPolicy: kp.pullPolicy,
			Ports:           []Port{{ContainerPort: 80}},
		},
	}
	return pod
}

func (kp *kubectlPortal) deleteExistingProxyPod() error {
	kc := newKubectl(
		"delete",
		"pod",
		proxyPodName(),
		"--ignore-not-found",
	).namespace(kp.namespace)

	out, err := kc.run(nil)
	if err != nil {
		return fmt.Errorf("'kubectl delete pod' failed: %w\n%v", err, string(out))
	}
	return nil
}

func (kp *kubectlPortal) createProxyPod() error {
	fmt.Println("creating proxy Pod...")

	data, err := json.Marshal(kp.proxyPod())
	if err != nil {
		panic(fmt.Sprintf("error: unable to marshal Pod data: %s", err))
	}

	kc := newKubectl("apply", "-f", "-").namespace(kp.namespace)
	out, err := kc.run(data)
	if err != nil {
		return fmt.Errorf("'kubectl apply' failed: %w\n%v", err, string(out))
	}
	return nil
}

func (kp *kubectlPortal) waitForProxyPod() error {
	fmt.Println("waiting for proxy Pod to be ready...")

	kc := newKubectl(
		"wait",
		"--for=condition=Ready",
		"pod/"+proxyPodName(),
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
		proxyPodName(),
		fmt.Sprintf("%v:80", kp.port),
	).namespace(kp.namespace)

	cmd, err := kc.start()
	if err != nil {
		return fmt.Errorf("'kubectl port-forward' failed: %w\n", err)
	}

	fmt.Printf("kubectl port-forward now running at localhost:%v\n", kp.port)

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
	err := kp.deleteExistingProxyPod()
	if err != nil {
		return err
	}

	err = kp.createProxyPod()
	if err != nil {
		return err
	}
	defer func() {
		err2 := kp.deleteExistingProxyPod()
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

func parseFlags(kp *kubectlPortal) error {
	help := false
	defaultImage := fmt.Sprintf("%v:%v", proxyPodImageBase, proxyPodImageVersion)
	defaultPodName := proxyPodName()

	flags := pflag.NewFlagSet("kubectl-portal", pflag.ContinueOnError)
	pflag.CommandLine = flags

	configFlags := genericclioptions.ConfigFlags{
		Namespace: &kp.namespace,
	}
	configFlags.AddFlags(flags)
	flags.BoolVarP(&help, "help", "h", false, "Show usage help")
	flags.UintVar(&kp.port, "portal-port", defaultPort, "Local port to use for HTTP proxy")
	flags.StringVar(&kp.image, "portal-image", defaultImage, "Image to use for HTTP proxy")
	flags.StringVar(&kp.proxyPodName, "portal-name", defaultPodName, "Pod name to use for HTTP proxy")
	flags.StringVar(&kp.pullPolicy, "portal-pull-policy", proxyPodImagePullPolicy, "Image pull policy to use for HTTP proxy")

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
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}

	err = kp.run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
}
