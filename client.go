package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

type Client interface {
	Build() error
	Push(deployId string) error
	// Run runs the specified deploy, returning the port it is listening on.
	Run(deployId string) error

	// Sets active deploy by port
	SetMainByPort(port int) error

	// Sets active deploy using the deploy id
	SetMainById(string) error

	ListDeploys() ([]*Deploy, error)
	Stop(deployId string) error
	KillUnknownProcesses()
	Shutdown()
}

type SingleServerClient struct {
	app    Application
	client *rpc.Client
	target *Target
	dir    string

	// In test mode, connect to a camus server running on the same machine, and
	// all 'remote commands' are executed locally (via bash), as opposed to via ssh
	isLocalTest bool
}

func NewClientImpl(deployFile string, targetName string, isLocalTest bool) (*SingleServerClient, error) {
	app, err := ApplicationFromConfig(true, deployFile)
	if err != nil {
		return nil, err
	}

	target := app.Target(targetName)
	if target == nil {
		return nil, fmt.Errorf("Invalid target: '%s'", targetName)
	}

	localPort := target.Base
	if !isLocalTest {
		localPort = setupChannel(target.Base, target.SshPort, target.Ssh)
	}

	serverAddr := fmt.Sprintf("localhost:%d", localPort)
	client, err := rpc.DialHTTP("tcp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("Dialing: %s", err)
	}

	return &SingleServerClient{
		app:         app,
		client:      client,
		target:      target,
		dir:         path.Dir(deployFile),
		isLocalTest: isLocalTest,
	}, nil
}

func (c *SingleServerClient) Build() error {
	cmd := exec.Command("sh", "-c", c.app.BuildCmd())
	cmd.Dir = c.dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func (c *SingleServerClient) Push(deployId string) error {
	req := &GetDeploysPathRequest{}
	var reply GetDeploysPathReply

	err := c.client.Call("RpcServer.GetDeploysPath", req, &reply)
	if err != nil {
		return err
	}

	c.info("uploading package...")

	localDeployDir := c.app.BuildOutputDir()
	remoteDeployDir := path.Join(reply.Path, deployId)
	remoteLatestDir := path.Join(remoteDeployDir, "../../_latest")

	// Base rsync command
	rsyncArgs := []string{
		"-azv", "--delete",
		localDeployDir + "/",
	}

	// Extra flags to rsync over ssh
	if c.isLocalTest {
		rsyncArgs = append(rsyncArgs, remoteLatestDir)
	} else {
		rsyncArgs = append(rsyncArgs, []string{
			"-e",
			fmt.Sprintf("ssh -p %d -o StrictHostKeyChecking=no", c.target.SshPort),
			c.target.Ssh + ":" + remoteLatestDir,
		}...)
	}

	if err := c.runVisibleCmd("rsync", rsyncArgs...); err != nil {
		return err
	}

	if err := c.runRemoteCmd("rsync", "-a", "--delete", remoteLatestDir+"/", remoteDeployDir); err != nil {
		return err
	}

	c.info("done uploading")

	postDeployCmd := c.app.PostDeployCmd()
	if postDeployCmd != "" {
		cmd := fmt.Sprintf("cd %s; %s", remoteDeployDir, postDeployCmd)
		if err := c.runRemoteCmd(cmd); err != nil {
			return err
		}
		c.info("post deploy command completed")
	}

	return nil
}

func (c *SingleServerClient) runRemoteCmd(command ...string) error {
	if c.isLocalTest {
		return c.runVisibleCmd("bash", "-c", strings.Join(command, " "))
	} else {
		sshArgs := []string{
			"-o", "StrictHostKeyChecking=no",
			"-p", strconv.Itoa(c.target.SshPort),
			c.target.Ssh,
		}
		args := append(sshArgs, command...)
		return c.runVisibleCmd("ssh", args...)
	}
}

func (c *SingleServerClient) runVisibleCmd(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = c.dir
	fmt.Printf("exec %s\n", cmd.Args)
	return cmd.Run()
}

func (c *SingleServerClient) Run(deployId string) error {
	req := &RunRequest{deployId}
	var reply RunReply
	err := c.client.Call("RpcServer.Run", req, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *SingleServerClient) Stop(deployId string) error {
	req := &StopDeployRequest{deployId}
	var reply StopDeployResponse
	return c.client.Call("RpcServer.StopDeploy", &req, &reply)
}

func (c *SingleServerClient) SetMainByPort(port int) error {
	req := &SetMainPortRequest{port}
	var reply SetMainPortReply
	err := c.client.Call("RpcServer.SetMainByPort", req, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *SingleServerClient) SetMainById(id string) error {
	req := &SetMainByIdRequest{id}
	var reply SetMainByIdReply
	err := c.client.Call("RpcServer.SetMainById", req, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *SingleServerClient) ListDeploys() ([]*Deploy, error) {
	args := &ListDeploysRequest{}
	var reply ListDeploysReply
	if err := c.client.Call("RpcServer.ListDeploys", args, &reply); err != nil {
		return nil, err
	}

	return reply.Deploys, nil
}

func (c *SingleServerClient) info(args ...interface{}) {
	log.Println(prepend("    client: ", args)...)
}

func prepend(item interface{}, items []interface{}) []interface{} {
	return append([]interface{}{item}, items...)
}

func getFreeLocalPort() (port int) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatalf("Couldn't get free local port (to setup ssh tunnel)", err)
	}
	parts := strings.Split(l.Addr().String(), ":")
	l.Close()

	port, err = strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		log.Fatalf("Couldn't parse port from", parts[len(parts)-1], err)
	}

	return
}

func setupChannel(remotePort int, sshPort int, login string) int {
	localPort := getFreeLocalPort()

	cmd := exec.Command(
		"ssh", "-o", "StrictHostKeyChecking=no", "-p", strconv.Itoa(sshPort), login,
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

func (c *SingleServerClient) KillUnknownProcesses() {
	var args KillUnknownProcessesRequest
	var reply KillUnknownProcessesResponse
	c.client.Call("RpcServer.KillUnknownProcesses", &args, &reply)
}

func (c *SingleServerClient) Shutdown() {
	var args ShutdownRequest
	var reply ShutdownResponse
	c.client.Call("RpcServer.Shutdown", &args, &reply)
}
