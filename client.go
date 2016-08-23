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

	SetActiveByPort(port int) error
	SetActiveById(string) error

	ListDeploys() ([]*Deploy, error)
	Stop(deployId string) error
	KillUnknownProcesses()
	CleanupDeploy(deployId string) error
	Shutdown()
}

type SingleServerClient struct {
	app    Application
	client *rpc.Client
	target *Target
	appDir string

	// In test mode, connect to a camus server running on the same machine, and
	// all 'remote commands' are executed locally (via bash), as opposed to via ssh
	isLocalTest bool
}

// Client which communicates with multiple underlying servers at once. Used if
// the target is actually a group of servers
type MultiServerClient struct {
	app     Application
	appDir  string
	clients []Client
}

func NewClient(deployFile string, targetName TargetName, isLocalTest bool) (*MultiServerClient, error) {
	app, err := ApplicationFromConfig(true, deployFile)
	if err != nil {
		return nil, err
	}

	// Create all SingleServerClients
	clients := []Client{}
	for _, target := range app.Targets(targetName) {
		localPort := target.Base
		if !isLocalTest {
			localPort = setupChannel(target.Base, target.SshPort, target.Ssh)
		}

		serverAddr := fmt.Sprintf("localhost:%d", localPort)
		client, err := rpc.DialHTTP("tcp", serverAddr)
		if err != nil {
			return nil, fmt.Errorf("Dialing: %s", err)
		}

		clients = append(clients, &SingleServerClient{
			app:         app,
			client:      client,
			target:      target,
			appDir:      path.Dir(deployFile),
			isLocalTest: isLocalTest,
		})
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("Invalid target: '%s'", targetName)
	}

	return &MultiServerClient{
		app:     app,
		appDir:  path.Dir(deployFile),
		clients: clients,
	}, nil
}

func (c *SingleServerClient) Build() error {
	return build(c.app.BuildCmd(), c.appDir)
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
	cmd.Dir = c.appDir
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

func (c *SingleServerClient) SetActiveByPort(port int) error {
	req := &SetActivePortRequest{port}
	var reply SetActivePortReply
	err := c.client.Call("RpcServer.SetActiveByPort", req, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *SingleServerClient) SetActiveById(deployId string) error {
	req := &SetActiveByIdRequest{deployId}
	var reply SetActiveByIdReply
	err := c.client.Call("RpcServer.SetActiveById", req, &reply)
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

func (c *SingleServerClient) CleanupDeploy(deployIdForCleanup string) error {
	req := &SetActiveByIdRequest{deployIdForCleanup}
	var reply SetActiveByIdReply
	return c.client.Call("RpcServer.CleanupDeploy", req, &reply)
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

// MultiServerClient

func (c *MultiServerClient) Build() error {
	return build(c.app.BuildCmd(), c.appDir)
}

func (c *MultiServerClient) Push(deployId string) error {
	for _, c := range c.clients {
		if err := c.Push(deployId); err != nil {
			return err
		}
	}

	return nil
}

func (c *MultiServerClient) Run(deployId string) error {
	for _, c := range c.clients {
		if err := c.Run(deployId); err != nil {
			return err
		}
	}

	return nil
}

func (c *MultiServerClient) Stop(deployId string) error {
	for _, c := range c.clients {
		if err := c.Stop(deployId); err != nil {
			return err
		}
	}

	return nil
}

func (c *MultiServerClient) SetActiveByPort(port int) error {
	// Only makes sense if you are connecting to a single backend
	// server
	if len(c.clients) > 1 {
		return fmt.Errorf("Cannot set current app by port when target is a group of machines")
	}

	return c.clients[0].SetActiveByPort(port)
}

func (c *MultiServerClient) SetActiveById(deployId string) error {
	for _, c := range c.clients {
		if err := c.SetActiveById(deployId); err != nil {
			return err
		}
	}

	return nil
}

func (c *MultiServerClient) ListDeploys() ([]*Deploy, error) {
	var deploys []*Deploy

	for _, c := range c.clients {
		if deploysForServer, err := c.ListDeploys(); err != nil {
			return nil, err
		} else {
			deploys = append(deploys, deploysForServer...)
		}
	}

	return deploys, nil
}

func (c *MultiServerClient) CleanupDeploy(deployIdForCleanup string) error {
	for _, c := range c.clients {
		if err := c.CleanupDeploy(deployIdForCleanup); err != nil {
			return err
		}
	}

	return nil
}

func (c *MultiServerClient) KillUnknownProcesses() {
	for _, c := range c.clients {
		c.KillUnknownProcesses()
	}
}

func (c *MultiServerClient) Shutdown() {
	for _, c := range c.clients {
		c.Shutdown()
	}
}

func build(buildCmd string, appDir string) error {
	cmd := exec.Command("sh", "-c", buildCmd)
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
