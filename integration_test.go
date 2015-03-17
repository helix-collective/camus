package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func run(t *testing.T, cmd string) *exec.Cmd {
	return runInDir(t, cmd, "")
}

func runInTestApp(t *testing.T, cmd string) *exec.Cmd {
	return runInDir(t, cmd, "testapp")
}

func runInDir(t *testing.T, cmdString, dir string) *exec.Cmd {
	words := strings.Split(cmdString, " ")
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	fmt.Printf("[%s]\n", cmdString)
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to run '%s': %s\n", cmd, err)
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
		t.Fatalf("Failed to start '%s': %s\n", cmd, err)
	}
	return cmd
}

func TestEverything(t *testing.T) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("%s", err)
	}
	deployDir, err := ioutil.TempDir("/tmp", "camustest-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %s\n", err)
	}
	run(t, "go build")
	server := startInDir(t, cwd+"/camus -server", deployDir)
	defer server.Process.Kill()
	time.Sleep(1 * time.Second)

	os.Chdir("testapp")

	client, err := NewClientImpl(".")
	if err != nil {
		t.Fatalf("%s", err)
	}

	_, err = client.Build()
	if err != nil {
		t.Fatalf("client build: %s", err)
	}
	oldDeploys, err := client.ListDeploys()
	if err != nil {
		t.Fatalf("client list deploys: %s", err)
	}
	deployId, err := client.Push("prod")
	if err != nil {
		t.Fatalf("client push: %s", err)
	}
	deploys, err := client.ListDeploys()
	if err != nil {
		t.Fatalf("client list deploys: %s", err)
	}
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

func findDeployById(deployId string, deploys []*Deploy) bool {
	for _, deploy := range deploys {
		if deploy.Id == deployId {
			return true
		}
	}
	return false
}
