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
	Shutdown()
}

type SingleTargetClient struct {
	app           Application
	client        *rpc.Client
	target        *Target
	appDir        string
	serverChannel TargetBox
}

// Client which communicates with multiple underlying servers at once. Used if
// the target is actually a group of servers
type MultiTargetClient struct {
	app     Application
	appDir  string
	clients []Client
}

func NewClient(deployFile string, targetName TargetName, isLocalTest bool) (*MultiTargetClient, error) {
	app, err := ApplicationFromConfig(true, deployFile)
	if err != nil {
		return nil, err
	}

	appDir := path.Dir(deployFile)

	// Create all SingleTargetClients
	clients := []Client{}
	for _, target := range app.Targets(targetName) {
		localPort := target.Base
		var serverChannel TargetBox

		if !isLocalTest {
			localPort = getFreeLocalPort()
			serverChannel, err = NewSshChannel(
				target.Base,
				localPort,
				target.SshPort,
				target.Ssh,
				newCommandRunner(appDir),
			)
			if err != nil {
				return nil, err
			}
		} else {
			serverChannel = NewLocalChannel(newCommandRunner(appDir))
		}

		serverAddr := fmt.Sprintf("localhost:%d", localPort)
		client, err := rpc.DialHTTP("tcp", serverAddr)
		if err != nil {
			return nil, fmt.Errorf("Dialing: %s", err)
		}

		clients = append(clients, &SingleTargetClient{
			app:           app,
			client:        client,
			target:        target,
			appDir:        appDir,
			serverChannel: serverChannel,
		})
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("Invalid target: '%s'", targetName)
	}

	return &MultiTargetClient{
		app:     app,
		appDir:  path.Dir(deployFile),
		clients: clients,
	}, nil
}

func (c *SingleTargetClient) Build() error {
	return build(c.app.BuildCmd(), c.appDir)
}

func (c *SingleTargetClient) Push(deployId string) error {
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

	if err := c.serverChannel.Copy(localDeployDir, remoteLatestDir); err != nil {
		return err
	}

	if err := c.serverChannel.Exec("rsync", "-a", "--delete", remoteLatestDir+"/", remoteDeployDir); err != nil {
		return err
	}

	c.info("done uploading")

	postDeployCmd := c.app.PostDeployCmd()
	if postDeployCmd != "" {
		cmd := fmt.Sprintf("cd %s; %s", remoteDeployDir, postDeployCmd)
		if err := c.serverChannel.Exec(cmd); err != nil {
			return err
		}
		c.info("post deploy command completed")
	}

	return nil
}

func (c *SingleTargetClient) Run(deployId string) error {
	req := &RunRequest{deployId}
	var reply RunReply
	err := c.client.Call("RpcServer.Run", req, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *SingleTargetClient) Stop(deployId string) error {
	req := &StopDeployRequest{deployId}
	var reply StopDeployResponse
	return c.client.Call("RpcServer.StopDeploy", &req, &reply)
}

func (c *SingleTargetClient) SetActiveByPort(port int) error {
	req := &SetActivePortRequest{port}
	var reply SetActivePortReply
	err := c.client.Call("RpcServer.SetActiveByPort", req, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *SingleTargetClient) SetActiveById(deployId string) error {
	req := &SetActiveByIdRequest{deployId}
	var reply SetActiveByIdReply
	err := c.client.Call("RpcServer.SetActiveById", req, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *SingleTargetClient) ListDeploys() ([]*Deploy, error) {
	args := &ListDeploysRequest{}
	var reply ListDeploysReply
	if err := c.client.Call("RpcServer.ListDeploys", args, &reply); err != nil {
		return nil, err
	}

	return reply.Deploys, nil
}

func (c *SingleTargetClient) info(args ...interface{}) {
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

func (c *SingleTargetClient) KillUnknownProcesses() {
	var args KillUnknownProcessesRequest
	var reply KillUnknownProcessesResponse
	c.client.Call("RpcServer.KillUnknownProcesses", &args, &reply)
}

func (c *SingleTargetClient) Shutdown() {
	var args ShutdownRequest
	var reply ShutdownResponse
	c.client.Call("RpcServer.Shutdown", &args, &reply)
}

// MultiTargetClient

func (c *MultiTargetClient) Build() error {
	return build(c.app.BuildCmd(), c.appDir)
}

func (c *MultiTargetClient) Push(deployId string) error {
	for _, c := range c.clients {
		if err := c.Push(deployId); err != nil {
			return err
		}
	}

	return nil
}

func (c *MultiTargetClient) Run(deployId string) error {
	for _, c := range c.clients {
		if err := c.Run(deployId); err != nil {
			return err
		}
	}

	return nil
}

func (c *MultiTargetClient) Stop(deployId string) error {
	for _, c := range c.clients {
		if err := c.Stop(deployId); err != nil {
			return err
		}
	}

	return nil
}

func (c *MultiTargetClient) SetActiveByPort(port int) error {
	// Only makes sense if you are connecting to a single backend
	// server
	if len(c.clients) > 1 {
		return fmt.Errorf("Cannot set current app by port when target is a group of machines")
	}

	return c.clients[0].SetActiveByPort(port)
}

func (c *MultiTargetClient) SetActiveById(deployId string) error {
	for _, c := range c.clients {
		if err := c.SetActiveById(deployId); err != nil {
			return err
		}
	}

	return nil
}

func (c *MultiTargetClient) ListDeploys() ([]*Deploy, error) {
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

func (c *MultiTargetClient) KillUnknownProcesses() {
	for _, c := range c.clients {
		c.KillUnknownProcesses()
	}
}

func (c *MultiTargetClient) Shutdown() {
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

func newCommandRunner(cwd string) CommandRunner {
	return func(command string, args ...string) error {
		cmd := exec.Command(command, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = cwd
		fmt.Printf("exec %s\n", cmd.Args)
		return cmd.Run()
	}
}
