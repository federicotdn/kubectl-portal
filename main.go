package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"os/signal"

	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	nameHashLength   = 10
	nginxPodNameBase = "kubectl-portal-nginx"
)

type stringMap map[string]string

type Port struct {
	ContainerPort int `json:"containerPort"`
}

type Container struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Ports []Port `json:"ports"`
}

type Pod struct {
	ApiVersion string    `json:"apiVersion"`
	Kind       string    `json:"kind"`
	Metadata   stringMap `json:"metadata"`
	Spec       struct {
		Containers []Container `json:"containers"`
	} `json:"spec"`
}

type kubectlCmd struct {
	args []string
}

func printErr(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
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

func nginxPodName() string {
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
	return nginxPodNameBase + "-" + hash[:nameHashLength]
}

func nginxPod() Pod {
	pod := Pod{
		ApiVersion: "v1",
		Kind:       "Pod",
		Metadata: stringMap{
			"name": nginxPodName(),
		},
	}
	pod.Spec.Containers = []Container{
		{
			Name:  "nginx",
			Image: "nginx",
			Ports: []Port{{ContainerPort: 80}},
		},
	}
	return pod
}

func deleteExistingNginxPod(namespace string) {
	kc := newKubectl(
		"delete",
		"pod",
		nginxPodName(),
		"--ignore-not-found",
	).namespace(namespace)

	out, err := kc.run(nil)
	if err != nil {
		printErr("error: 'kubectl delete pod' failed: %s\n%s", err, string(out))
	}
}

func createNginxPod(namespace string) {
	data, err := json.Marshal(nginxPod())
	if err != nil {
		panic(fmt.Sprintf("error: unable to marshal Pod data: %s", err))
	}

	kc := newKubectl("apply", "-f", "-").namespace(namespace)
	out, err := kc.run(data)
	if err != nil {
		printErr("error: 'kubectl apply' failed: %s\n%s", err, string(out))
	}
}

func waitForNginxPod(namespace string) {
	kc := newKubectl(
		"wait",
		"--for=condition=Ready",
		"pod/" + nginxPodName(),
	).namespace(namespace)

	out, err := kc.run(nil)
	if err != nil {
		printErr("error: 'kubectl wait' failed: %s\n%s", err, string(out))
	}
}

func portForwardNginxPod(namespace string) {
	kc := newKubectl(
		"port-forward",
		nginxPodName(),
		"8080:80",
	).namespace(namespace)

	cmd, err := kc.start()
	if err != nil {
		printErr("error: 'kubectl port-forward' failed: %s\n", err)
	}

	fmt.Println("kubectl port-forward now running...")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	fmt.Println("\ninterrupt received")

	err = cmd.Process.Signal(os.Interrupt)
	if err != nil {
		printErr("error: interrupt 'kubectl port-forward' failed: %s\n", err)
	}

	err = cmd.Wait()
	if err != nil {
		printErr("error: wait 'kubectl port-forward' failed: %s\n", err)
	}
}

func main() {
	fmt.Println("starting")

	help := false
	namespace := ""
	flags := pflag.NewFlagSet("kubectl-portal", pflag.ContinueOnError)
	pflag.CommandLine = flags

	configFlags := genericclioptions.ConfigFlags{
		Namespace: &namespace,
	}
	configFlags.AddFlags(flags)
	flags.BoolVarP(&help, "help", "h", false, "Show usage help")

	err := flags.Parse(os.Args)
	if err != nil {
		printErr("error: %v\nSee 'kubectl portal --help' for usage.\n", err)
	}

	if help {
		fmt.Printf("Options:\n%v", flags.FlagUsages())
		return
	}

	deleteExistingNginxPod(namespace)
	createNginxPod(namespace)
	waitForNginxPod(namespace)
	portForwardNginxPod(namespace)
	deleteExistingNginxPod(namespace)
}
