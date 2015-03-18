package main

import (
	"fmt"
	"log"
	"net/rpc"
	"os"
	"os/exec"
	"path"
	"strconv"
)

type Client interface {
	Build() (string, error)
	Push() (string, error)
	Run(deployId string) (int, error)
	SetMainByPort(port int) error
	ListDeploys() ([]*Deploy, error)
	KillUnknownProcesses()
	Shutdown()
}

type ClientImpl struct {
	app    Application
	client *rpc.Client
	target *Target
}

func NewClientImpl(deployFile string, targetName string) (*ClientImpl, error) {
	app, err := ApplicationFromConfig(true, deployFile)
	if err != nil {
		return nil, err
	}

	target := app.Target(targetName)
	if target == nil {
		return nil, fmt.Errorf("Invalid target: '%s'", targetName)
	}

	localPort := setupChannel(target.Base, target.SshPort, target.Ssh)

	serverAddr := fmt.Sprintf("localhost:%d", localPort)
	client, err := rpc.DialHTTP("tcp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("Dialing: %s", err)
	}

	return &ClientImpl{
		app:    app,
		client: client,
		target: target,
	}, nil
}

func (c *ClientImpl) Build() (string, error) {
	cmd := exec.Command("sh", "-c", c.app.BuildCmd())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return "dummy", nil
}

func (c *ClientImpl) Push() (string, error) {
	req := &NewDeployDirRequest{}
	var reply NewDeployDirResponse

	err := c.client.Call("RpcServer.NewDeployDir", req, &reply)
	if err != nil {
		return "", err
	}

	c.info("uploading package...")

	localDeployDir := c.app.BuildOutputDir()
	remoteDeployDir := reply.Path
	remoteLatestDir := path.Join(remoteDeployDir, "../../_latest")

	// TODO(koz): Delete this code when I fix ssh on my computer.
	/*
		if true {
			err := runVisibleCmd("rsync", "-azv", "--delete",
				localDeployDir+"/",
				remoteDeployDir)
			if err != nil {
				return "", err
			}
			return reply.DeployId, nil
		}
	*/
	if err := runVisibleCmd("rsync", "-azv", "--delete",
		localDeployDir+"/",
		"-e", fmt.Sprintf("ssh -p %d", c.target.SshPort),
		c.target.Ssh+":"+remoteLatestDir); err != nil {
		return "", err
	}

	if err := runVisibleCmd(
		"ssh", "-p", strconv.Itoa(c.target.SshPort), c.target.Ssh,
		"rsync", "-a", "--delete",
		remoteLatestDir+"/", remoteDeployDir); err != nil {
		return "", err
	}

	c.info("done uploading")

	return reply.DeployId, nil
}

func runVisibleCmd(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("exec %s\n", cmd.Args)
	return cmd.Run()
}

func (c *ClientImpl) Run(deployId string) (int, error) {
	req := &RunRequest{deployId}
	var reply RunReply
	err := c.client.Call("RpcServer.Run", req, &reply)
	if err != nil {
		return -1, err
	}

	return reply.Port, nil
}

func (c *ClientImpl) SetMainByPort(port int) error {
	req := &SetMainPortRequest{port}
	var reply SetMainPortReply
	err := c.client.Call("RpcServer.SetMainByPort", req, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientImpl) ListDeploys() ([]*Deploy, error) {
	args := &ListDeploysRequest{}
	var reply ListDeploysReply
	if err := c.client.Call("RpcServer.ListDeploys", args, &reply); err != nil {
		return nil, err
	}

	return reply.Deploys, nil
}

func (c *ClientImpl) info(args ...interface{}) {
	log.Println(prepend("    client: ", args)...)
}

func prepend(item interface{}, items []interface{}) []interface{} {
	return append([]interface{}{item}, items...)
}

func setupChannel(remotePort int, sshPort int, login string) int {
	localPort := 9847 // todo: Just get some free port
	cmd := exec.Command(
		"ssh", "-p", strconv.Itoa(sshPort), login,
		fmt.Sprintf("-L%d:localhost:%d", localPort, remotePort))
	_, err := cmd.StdinPipe()
	err = cmd.Start()

	if err != nil {
		log.Fatalf("%s", err)
	}

	fmt.Printf("Opening connection to %s:%d -> camus@%d ..",
		login, sshPort, remotePort)

	for portFree(localPort) {
		print(".")
		sleepSeconds(1)
	}
	println()

	return localPort
}

func (c *ClientImpl) KillUnknownProcesses() {
	var args KillUnknownProcessesRequest
	var reply KillUnknownProcessesResponse
	c.client.Call("RpcServer.KillUnknownProcesses", &args, &reply)
}

func (c *ClientImpl) Shutdown() {
	var args ShutdownRequest
	var reply ShutdownResponse
	c.client.Call("RpcServer.Shutdown", &args, &reply)
}
