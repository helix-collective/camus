package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testClient struct {
	t      *testing.T
	client Client
}

func (tc *testClient) Build() {
	_, err := tc.client.Build()
	if err != nil {
		tc.t.Fatalf("client build: %s", err)
	}
}

func (tc *testClient) ListDeploys() []*Deploy {
	deploys, err := tc.client.ListDeploys()
	if err != nil {
		tc.t.Fatalf("client list deploys: %s", err)
	}
	return deploys
}

func (tc *testClient) Push() string {
	deployId, err := tc.client.Push()
	if err != nil {
		tc.t.Fatalf("client push: %s", err)
	}
	return deployId
}

func (tc *testClient) Shutdown() {
	tc.client.Shutdown()
}

func (tc *testClient) Run(deployId string) int {
	port, err := tc.client.Run(deployId)
	if err != nil {
		tc.t.Fatalf("client run: %s\n", err)
	}
	return port
}

func (tc *testClient) SetMainByPort(port int) {
	err := tc.client.SetMainByPort(port)
	if err != nil {
		tc.t.Fatalf("set main by port: %s\n", err)
	}
}

func run(t *testing.T, cmd string) *exec.Cmd {
	return runInDir(t, cmd, "")
}

func runInDir(t *testing.T, cmdString, dir string) *exec.Cmd {
	words := strings.Split(cmdString, " ")
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	fmt.Printf("[%s]\n", cmdString)
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to run '%s': %s\n", cmd.Args, err)
	}
	return cmd
}

func startInDir(t *testing.T, cmdString, dir string) *exec.Cmd {
	words := strings.Split(cmdString, " ")
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	fmt.Printf("[%s]\n", cmdString)
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start '%s': %s\n", cmd.Args, err)
	}
	return cmd
}

func findDeployById(deployId string, deploys []*Deploy) bool {
	for _, deploy := range deploys {
		if deploy.Id == deployId {
			return true
		}
	}
	return false
}

func startCamus(t *testing.T) (*testClient, *os.Process) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("%s", err)
	}
	deployDir, err := ioutil.TempDir("/tmp", "camustest-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %s\n", err)
	}
	run(t, "go build")

	// We start a new server process instead of running it here to avoid the
	// complexities of shutting down an HTTP server in-process in go.
	server := startInDir(t, cwd+"/camus -server", deployDir)
	// Give the server time to start up.
	time.Sleep(1 * time.Second)

	client, err := NewClientImpl("testapp/deploy.json", "prod")
	if err != nil {
		t.Fatalf("%s", err)
	}
	return &testClient{t, client}, server.Process
}

func TestDeploy(t *testing.T) {
	client, server := startCamus(t)
	// client.Shutdown() should kill the server process, but if that fails we
	// want to kill it. No harm in killing it if it's already dead, too.
	// TODO(koz): Kill everything listening on the given port range as a final cleanup step.
	// TODO(koz): Make camus able to run on a specified port range (dynamically found?)
	defer server.Kill()
	defer client.Shutdown()

	client.Build()
	oldDeploys := client.ListDeploys()
	deployId := client.Push()
	deploys := client.ListDeploys()
	if findDeployById(deployId, oldDeploys) {
		t.Fatal("newly minted deploy shouldn't have been in the old deploy list")
	}
	if !findDeployById(deployId, deploys) {
		t.Fatal("newly minted deploy should be in the new deploy list")
	}
	if len(deploys) != len(oldDeploys)+1 {
		t.Fatalf("expected %d deploys, but got %d", len(oldDeploys)+1, len(deploys))
	}
}

func getLocalhost(t *testing.T, port int, path string) string {
	// Don't reuse TCP connections as they may be to an old haproxy.
	http.DefaultTransport.(*http.Transport).DisableKeepAlives = true
	url := fmt.Sprintf("http://localhost:%d%s", port, path)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRun(t *testing.T) {
	client, server := startCamus(t)
	defer server.Kill()
	defer client.Shutdown()

	client.Build()
	deployId := client.Push()
	port := client.Run(deployId)
	data := getLocalhost(t, port, "")
	expected := "Hello World!"
	if string(data) != expected {
		t.Fatalf("expected %s, got %s", expected, data)
	}
}

func writeDataIntoTestapp(t *testing.T, data string) {
	err := ioutil.WriteFile("testapp/data/file", []byte(data), os.FileMode(0644))
	if err != nil {
		t.Fatalf("write file: %s\n", err)
	}
}

func expectGet(t *testing.T, port int, path, expected string) {
	data := getLocalhost(t, port, path)
	if data != expected {
		url := fmt.Sprintf("http://localhost:%d%s", port, path)
		t.Fatalf("expected %s to yield %s, but was %s", url, expected, data)
	}
}

func TestHaproxy(t *testing.T) {
	client, server := startCamus(t)
	defer server.Kill()
	defer client.Shutdown()

	writeDataIntoTestapp(t, "version 1")
	client.Build()
	v1DeployId := client.Push()

	// XXX(koz): Hilariously, as we choose ids based on the current timestamp
	// to the second, pushing twice in quick succession here breaks as we
	// generate the same deployId. For now fix by waiting a second, but
	// obviously we should do something smarter like accept a label, or add a
	// .2 suffix, etc.
	time.Sleep(1 * time.Second)

	writeDataIntoTestapp(t, "version 2")
	client.Build()
	v2DeployId := client.Push()

	port1 := client.Run(v1DeployId)
	port2 := client.Run(v2DeployId)

	expectGet(t, port1, "/file", "version 1")
	expectGet(t, port2, "/file", "version 2")

	// TODO(koz): Don't hardcode the haproxy port.
	haproxyPort := 8098
	client.SetMainByPort(port2)
	expectGet(t, haproxyPort, "/file", "version 2")
	client.SetMainByPort(port1)
	expectGet(t, haproxyPort, "/file", "version 1")
	client.SetMainByPort(port2)
	expectGet(t, haproxyPort, "/file", "version 2")
}

// TestTracked tests that the Tracked field is only true when a deploy is
// configured to run.
func TestTracked(t *testing.T) {
	client, server := startCamus(t)
	defer server.Kill()
	defer client.Shutdown()

	client.Build()
	deployId := client.Push()
	deploys := client.ListDeploys()
	if len(deploys) != 1 {
		t.Fatalf("expected exactly 1 deploy, got %d", len(deploys))
	}
	if deploys[0].Tracked {
		t.Fatalf("expected the deploy not to be tracked, but it was")
	}

	client.Run(deployId)
	deploys = client.ListDeploys()
	if len(deploys) != 1 {
		t.Fatalf("expected exactly 1 deploy, got %d", len(deploys))
	}
	if !deploys[0].Tracked {
		t.Fatalf("expected the deploy to be tracked, but it wasn't")
	}

	proc, err := os.FindProcess(deploys[0].Pid)
	if err != nil {
		t.Fatalf("failed to find the server process")
	}

	proc.Kill()

	deploys = client.ListDeploys()
	if len(deploys) != 1 {
		t.Fatalf("expected exactly 1 deploy, got %d", len(deploys))
	}
	if !deploys[0].Tracked {
		t.Fatalf("expected the deploy to be tracked, but it wasn't")
	}
}

// TestPort tests that if a deploy is configured to run on a certain port, but
// isn't for whatever reason, the port still shows up in camus list.
func TestPort(t *testing.T) {
	client, server := startCamus(t)
	defer server.Kill()
	defer client.Shutdown()

	client.Build()
	deployId := client.Push()
	port := client.Run(deployId)
	deploys := client.ListDeploys()
	if len(deploys) != 1 {
		t.Fatalf("expected exactly 1 deploy, got %d", len(deploys))
	}
	if deploys[0].Port != port {
		t.Fatalf("expected port to be %d, but is %d", port, deploys[0].Port)
	}
	if deploys[0].Health != 200 {
		t.Fatalf("expected health to be %d, but is %d", 200, deploys[0].Health)
	}

	proc, err := os.FindProcess(deploys[0].Pid)
	if err != nil {
		t.Fatalf("failed to find the server process")
	}
	proc.Kill()

	deploys = client.ListDeploys()
	if len(deploys) != 1 {
		t.Fatalf("expected exactly 1 deploy, got %d", len(deploys))
	}
	if deploys[0].Port != port {
		t.Fatalf("expected port to be %d, but is %d", port, deploys[0].Port)
	}
	if deploys[0].Health != 0 {
		t.Fatalf("expected health to be 0, but is %d", deploys[0].Health)
	}
}
