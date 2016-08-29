package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type testClient struct {
	t             *testing.T
	client        Client
	remoteRootDir string
}

func (tc *testClient) Build() {
	if err := tc.client.Build(); err != nil {
		tc.t.Fatalf("client build: %s", err)
	}
}

func (tc *testClient) ListDeploys() []*Deploy {
	return tc.ListDeploysWithRegex("")
}

func (tc *testClient) ListDeploysWithRegex(regex string) []*Deploy {
	deploys, err := tc.client.ListDeploys(regex)
	if err != nil {
		tc.t.Fatalf("client list deploys: %s", err)
	}
	return deploys
}

func (tc *testClient) Push() string {
	deployId := NewDeployId()
	err := tc.client.Push(deployId)
	if err != nil {
		tc.t.Fatalf("client push: %s", err)
	}
	return deployId
}

func (tc *testClient) Shutdown() {
	tc.client.Shutdown()
}

func (tc *testClient) Run(deployId string) {
	if err := tc.client.Run(deployId); err != nil {
		tc.t.Fatalf("client run: %s\n", err)
	}
}

func (tc *testClient) Stop(deployId string) {
	err := tc.client.Stop(deployId)
	if err != nil {
		tc.t.Fatalf("client stop: %s\n", err)
	}
}

func (tc *testClient) SetActiveByPort(port int) {
	err := tc.client.SetActiveByPort(port)
	if err != nil {
		tc.t.Fatalf("set main by port: %s\n", err)
	}
}

func (tc *testClient) SetActiveById(id string) {
	err := tc.client.SetActiveById(id)
	if err != nil {
		tc.t.Fatalf("set main by id: %s\n", err)
	}
}

func (tc *testClient) Cleanup(id string) {
	err := tc.client.CleanupDeploy(id)
	if err != nil {
		tc.t.Fatalf("client cleanup: %s", err)
	}
}

// For each matching deploy, ensure
//   1. It's running
//   2. The result of performing a GET query on 'path' is as expected
func (tc *testClient) checkDeploy(deployId string, path string, expected string) {
	deploys := tc.ListDeploys()

	found := false
	for _, d := range deploys {
		if d.Id == deployId {
			found = true

			if d.Port == -1 {
				tc.t.Fatalf("Deploy %s has no assigned port (unexpected)", deployId)
			}

			data := getLocalhost(tc.t, d.Port, path)
			if string(data) != expected {
				tc.t.Fatalf("Query on '%s' for deploy %s (running on port %d) failed. Expected %s, got %s", path, deployId, d.Port, expected, data)
			}
		}
	}

	if !found {
		tc.t.Fatalf("no deploy with id %s found", deployId)
	}
}

func (tc *testClient) pushAndRun() string {
	deployId := tc.Push()
	tc.Run(deployId)
	return deployId
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

func findDeployPort(deployId string, deploys []*Deploy) int {
	for _, deploy := range deploys {
		if deploy.Id == deployId {
			return deploy.Port
		}
	}
	return -1
}

func findDeployPid(deployId string, deploys []*Deploy) int {
	for _, deploy := range deploys {
		if deploy.Id == deployId {
			return deploy.Pid
		}
	}
	return 0
}

func startCamus(t *testing.T) (*testClient, *os.Process) {
	return startCamusWithConfig(t, "prod", "testapp/deploy.json")
}

func startCamusWithConfig(t *testing.T, targetName TargetName, confFile string) (*testClient, *os.Process) {
	run(t, "go build")

	deployDir := createTempDir(t)
	server := startServer(t, deployDir, targetName, confFile)
	client := startClient(t, deployDir, targetName, confFile)

	return client, server
}

func createTempDir(t *testing.T) string {
	deployDir, err := ioutil.TempDir("/tmp", "camustest-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %s\n", err)
	}

	return deployDir
}

func startClient(
	t *testing.T,
	remoteRootDir string,
	targetName TargetName,
	conf string) *testClient {

	isLocalTest := true
	client, err := NewClient(conf, targetName, isLocalTest)
	if err != nil {
		t.Fatalf("%s", err)
	}

	return &testClient{t, client, remoteRootDir}
}

func startServer(
	t *testing.T,
	deployDir string,
	name TargetName,
	confFile string) *os.Process {

	var def ApplicationDef

	data, err := ioutil.ReadFile(confFile)
	if err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(data, &def); err != nil {
		t.Fatalf("%s, Invalid json %s", confFile, err)
	}

	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("%s", err)
	}

	if _, ok := def.Targets[name]; !ok {
		t.Fatalf("No target with name %s in %s", name, confFile)
	}

	// We start a new server process instead of running it here to avoid the
	// complexities of shutting down an HTTP server in-process in go.
	server := startInDir(
		t,
		fmt.Sprintf("%s/camus -server -port %d", cwd, def.Targets[name].Base),
		deployDir)

	// Give the server time to start up.
	time.Sleep(1 * time.Second)

	return server.Process
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

	//also test the post deploy cmd was run
	yyyeh, err := ioutil.ReadFile(client.remoteRootDir + "/deploys/" + deployId + "/yyyeh")
	if err != nil {
		t.Fatal(err)
	} else if string(yyyeh) != "woo\n" {
		t.Fatalf("PostDeployCmd failure, expected %s to %s", "woo", yyyeh)
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
	client.Run(deployId)
	client.checkDeploy(deployId, "", "Hello World!")
}

func TestPidRun(t *testing.T) {
	client, server := startCamusWithConfig(t, "prod", "testapp/deploy2.json")
	defer server.Kill()
	defer client.Shutdown()

	expected := "Hello World!"

	client.Build()
	deployId := client.Push()
	client.Run(deployId)
	client.checkDeploy(deployId, "", expected)

	//TODO(tim) nice way to test Enforce() in out-of-process server? :s
	deploys := client.ListDeploys()
	var node *Deploy
	var testappHaproxy *Deploy
	for _, d := range deploys {
		fmt.Printf(" - %s on %d @ %d - %s\n", d.Id, d.Port, d.Pid, d)
		//deploy = frontend haproxy
		if regexp.MustCompile("\\d\\d\\d\\d-\\d\\d-\\d\\d-\\d\\d-\\d\\d-\\d\\d").MatchString(d.Id) {
			if testappHaproxy != nil {
				t.Fatalf("multiple testapp haproxy instances running %s and %s", d.Id, testappHaproxy.Id)
			}
			testappHaproxy = d

		} else if regexp.MustCompile("node-\\d+").MatchString(d.Id) {
			//backend deploy app = original process, but overridden by haproxy app pid
			if node != nil {
				t.Fatalf("multiple testapp node deploys running %s and %s", d.Id, node.Id)
			}
			node = d
		}
	}

	if node == nil || testappHaproxy == nil {
		t.Fatalf("expected to find both testapp node and testapp haproxy instances")
	}

	if node.Port != testappHaproxy.Port+20 {
		t.Fatalf("expected testapp haproxy port %d to be at 20 port offset from node %d", testappHaproxy.Port, node.Port)
	}

	data := getLocalhost(t, testappHaproxy.Port, "")
	if string(data) != expected {
		t.Fatalf("Query result on %s via haproxy failed. Expected %s, got %s", deployId, expected, data)
	}
}

func TestStop(t *testing.T) {
	client, server := startCamus(t)
	defer server.Kill()
	defer client.Shutdown()

	client.Build()
	deployId := client.Push()

	if err := client.client.Stop(deployId); err == nil {
		t.Fatalf("expected error when stopping non-running deploy")
	}

	client.Run(deployId)

	if err := client.client.Stop("something made up"); err == nil {
		t.Fatalf("expected error when stopping non-existent deploy")
	}

	client.checkDeploy(deployId, "", "Hello World!")

	// get port, so we can check later that the running process was actually
	// stopped
	found := false
	port := -1
	for _, d := range client.ListDeploys() {
		if d.Id == deployId && found {
			t.Fatalf("Multiple matches for deploy id %s (expected 1)", deployId)
		}

		if d.Id == deployId {
			found = true
			port = d.Port
		}
	}

	if !found {
		t.Fatalf("no deploy with id %s found. Shouldn't disappear from deploy list (should just be stopped)", deployId)
	}

	client.Stop(deployId)

	// Don't reuse TCP connections as they may be to an old haproxy.
	http.DefaultTransport.(*http.Transport).DisableKeepAlives = true
	url := fmt.Sprintf("http://localhost:%d", port)
	if _, geterr := http.Get(url); geterr == nil {
		t.Fatalf("process not stopped")
	} else if !strings.Contains(geterr.Error(), "connection reset by peer") {
		t.Fatalf("something failed, but it wasn't a reset connection: %s", geterr)
	}
}

func atomicWriteFile(data string, filename string) error {
	tempFile, err := ioutil.TempFile(os.TempDir(), "tempData")
	if err != nil {
		return fmt.Errorf("could not create temp file: %s\n", err)
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = tempFile.Write([]byte(data))
	if err != nil {
		return fmt.Errorf("Could not write to temp file: %s\n", err)
	}

	err = os.Rename(tempFile.Name(), filename)
	if err != nil {
		return fmt.Errorf("Could not move temp file to testapp: %s\n", err)
	}

	return nil
}

func writeDataIntoTestapp(t *testing.T, data string) {
	if err := atomicWriteFile(data, "testapp/data/file"); err != nil {
		t.Fatalf("%s", err)
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

	writeDataIntoTestapp(t, "version 2")
	client.Build()
	v2DeployId := client.Push()

	client.Run(v1DeployId)
	client.Run(v2DeployId)

	client.checkDeploy(v1DeployId, "/file", "version 1")
	client.checkDeploy(v2DeployId, "/file", "version 2")

	// TODO(koz): Don't hardcode the haproxy port.
	haproxyPort := 8098
	client.SetActiveById(v1DeployId)
	expectGet(t, haproxyPort, "/file", "version 1")
	client.SetActiveById(v2DeployId)
	expectGet(t, haproxyPort, "/file", "version 2")
	client.SetActiveById(v1DeployId)
	expectGet(t, haproxyPort, "/file", "version 1")

	// Same test as above set via port instead of deploy id
	v1Port := -1
	v2Port := -1
	for _, d := range client.ListDeploys() {
		if d.Id == v1DeployId {
			v1Port = d.Port
		}

		if d.Id == v2DeployId {
			v2Port = d.Port
		}
	}
	client.SetActiveByPort(v2Port)
	expectGet(t, haproxyPort, "/file", "version 2")
	client.SetActiveByPort(v1Port)
	expectGet(t, haproxyPort, "/file", "version 1")
	client.SetActiveByPort(v2Port)
	expectGet(t, haproxyPort, "/file", "version 2")
}

func TestMultiserver(t *testing.T) {
	run(t, "go build")

	deployDir := createTempDir(t)
	server1 := startServer(t, deployDir, "sydney-az1", "testapp/deploy.json")
	server2 := startServer(t, deployDir, "sydney-az2", "testapp/deploy.json")
	client := startClient(t, deployDir, "sydney-multiserver", "testapp/deploy.json")

	defer server1.Kill()
	defer server2.Kill()
	defer client.Shutdown()

	client.Build()
	deployId := client.Push()
	deploys := client.ListDeploys()
	if len(deploys) != 2 {
		t.Fatalf("expected exactly 2 deploys, got %d", len(deploys))
	}

	client.Run(deployId)

	deploys = client.ListDeploys()
	if len(deploys) != 2 {
		t.Fatalf("expected exactly 2 deploys after running %s, got %d", deployId, len(deploys))
	}

	// All deploys should be running
	for _, d := range deploys {
		_, err := os.FindProcess(d.Pid)
		if err != nil {
			t.Fatalf("failed to find the server process")
		}
	}
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
	client.Run(deployId)
	deploys := client.ListDeploys()
	if len(deploys) != 1 {
		t.Fatalf("expected exactly 1 deploy, got %d", len(deploys))
	}
	port := deploys[0].Port
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

func TestCleanup(t *testing.T) {
	client, server := startCamus(t)
	defer server.Kill()
	defer client.Shutdown()
	client.Build()

	// deploy without running and then cleanup
	deployOnly := client.Push()
	if !findDeployById(deployOnly, client.ListDeploys()) {
		t.Fatal("newly minted deploy should be in the new deploy list")
	}
	client.Cleanup(deployOnly)
	if findDeployById(deployOnly, client.ListDeploys()) {
		t.Fatal("deploy should have been cleaned up")
	}

	// deploy, run then attempt cleanup. then stop and cleanup
	deployStop := client.Push()
	client.Run(deployStop)
	if err := client.client.CleanupDeploy(deployStop); err == nil {
		t.Fatal("expected error when cleaning up a running deploy")
	}
	if !findDeployById(deployStop, client.ListDeploys()) {
		t.Fatal("deployStop should still be there after failed cleanup")
	}
	client.Stop(deployStop)
	client.Cleanup(deployStop)
	if findDeployById(deployStop, client.ListDeploys()) {
		t.Fatal("deploy should have been cleaned up")
	}

	// cleanup something non-existent
	if err := client.client.CleanupDeploy("non-existent-deploy"); err == nil {
		t.Fatal("expected error when cleaning up a non-existent-deploy")
	}

	// deploy, run then attempt cleanup. then kill process and cleanup
	deployKill := client.Push()
	client.Run(deployKill)
	if err := client.client.CleanupDeploy(deployKill); err == nil {
		t.Fatal("expected error when cleaning up a running deploy")
	}
	if !findDeployById(deployKill, client.ListDeploys()) {
		t.Fatal("deployKill should still be there after failed cleanup")
	}
	pid := findDeployPid(deployKill, client.ListDeploys())
	if pid == 0 {
		t.Fatal("could not find running process to kill")
	}
	// FindProcess never returns error on unix. no point checking
	proc, _ := os.FindProcess(pid)
	proc.Kill()
	client.Cleanup(deployKill)
	if findDeployById(deployKill, client.ListDeploys()) {
		t.Fatal("deploy should have been cleaned up")
	}

	// deploy then cleanup and then do a new deploy to see if it runs on the same port
	// it should run on the same port since it should be the lowest free port number
	deployA := client.Push()
	client.Run(deployA)
	portA := findDeployPort(deployA, client.ListDeploys())
	if portA <= 0 {
		t.Fatal("Could not find port number of deploy")
	}
	client.Stop(deployA)
	client.Cleanup(deployA)
	if findDeployById(deployA, client.ListDeploys()) {
		t.Fatal("Deploy should have been cleaned up")
	}
	deployB := client.Push()
	client.Run(deployB)
	portB := findDeployPort(deployB, client.ListDeploys())
	if portB != portA {
		t.Fatalf("ports should be the same but instead were %d and %d", portA, portB)
	}

}

func TestListRegex(t *testing.T) {
	client, server := startCamus(t)
	defer server.Kill()
	defer client.Shutdown()
	client.Build()

	deployA := client.pushAndRun()
	deployB := client.pushAndRun()
	deployC := client.pushAndRun()
	deployD := client.pushAndRun()

	deploysA := client.ListDeploysWithRegex(deployA)
	if len(deploysA) != 1 {
		t.Fatalf("Expected only one deploy with matching string, got %d", len(deploysA))
	}
	if deployA != deploysA[0].Id {
		t.Fatalf("Expected deployId %s got %s", deployA, deploysA[0].Id)
	}

	deploysAB := client.ListDeploysWithRegex(fmt.Sprintf("(%s|%s)", deployA, deployB))
	if len(deploysAB) != 2 {
		t.Fatalf("Expected only two deploys with matching string, got %d", len(deploysA))
	}
	if deployA != deploysAB[0].Id && deployA != deploysAB[1].Id {
		t.Fatalf("Expected deployId %s got %s and %s", deployA, deploysAB[0].Id, deploysAB[1].Id)
	}
	if deployB != deploysAB[0].Id && deployB != deploysAB[1].Id {
		t.Fatalf("Expected deployId %s got %s and %s", deployB, deploysAB[0].Id, deploysAB[1].Id)
	}

	deploysCD := client.ListDeploysWithRegex(fmt.Sprintf("(%s|%s)", deployC[2:], deployD[2:]))
	if len(deploysCD) != 2 {
		t.Fatalf("Expected only two deploys with matching string, got %d", len(deploysA))
	}
	if deployC != deploysCD[0].Id && deployC != deploysCD[1].Id {
		t.Fatalf("Expected deployId %s got %s and %s", deployC, deploysCD[0].Id, deploysCD[1].Id)
	}
	if deployD != deploysCD[0].Id && deployD != deploysCD[1].Id {
		t.Fatalf("Expected deployId %s got %s and %s", deployD, deploysCD[0].Id, deploysCD[1].Id)
	}

	deploysD := client.ListDeploysWithRegex(fmt.Sprintf("(%saaa|%s)", deployC[2:], deployD[2:]))
	if len(deploysD) != 1 {
		t.Fatalf("Expected only one deploy with matching string, got %d", len(deploysA))
	}
	if deployD != deploysD[0].Id {
		t.Fatalf("Expected deployId %s got %s", deployD, deploysD[0].Id)
	}

	client.Stop(deployB)
	client.Cleanup(deployB)

	deploysAB = client.ListDeploysWithRegex(fmt.Sprintf("(%s|%s)", deployA, deployB))
	if len(deploysAB) != 1 {
		t.Fatalf("Expected only one deploy with matching string, got %d", len(deploysA))
	}
	if deployA != deploysA[0].Id {
		t.Fatalf("Expected deployId %s got %s", deployA, deploysA[0].Id)
	}
}
