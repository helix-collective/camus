package main

import (
	"fmt"
	"log"
	"net/rpc"
	"os"
	"os/exec"
)

type Client interface {
	Push(server string) (string, error)
	Run(deployId string) (int, error)
	List() ([]Deploy, error)
}

type ClientImpl struct {
	app    Application
	client *rpc.Client
}

func NewClientImpl() *ClientImpl {
	app, err := ApplicationFromConfig("deploy.json")
	if err != nil {
		log.Fatal("Failed to read deploy config: ", err)
	}

	localPort := CAMUS_PORT

	serverAddr := fmt.Sprintf("localhost:%d", localPort)
	fmt.Printf("dialing %s\n", serverAddr)
	client, err := rpc.DialHTTP("tcp", serverAddr)
	if err != nil {
		log.Fatal("dialing:", err)
	}

	return &ClientImpl{
		app:    app,
		client: client,
	}
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

func (c *ClientImpl) Push(server string) (string, error) {
	req := &NewDeployDirRequest{}
	var reply NewDeployDirResponse

	err := c.client.Call("RpcServer.NewDeployDir", req, &reply)
	if err != nil {
		return "", err
	}

	target := fmt.Sprintf("%s:%s", c.app.SshTarget(server), reply.Path)

	cmd := exec.Command("rsync", "-az",
		fmt.Sprintf("%s/", c.app.BuildOutputDir()),
		target)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	c.info("uploading package...")
	if err := cmd.Run(); err != nil {
		return "", err
	}
	c.info("done uploading")

	return reply.DeployId, nil
}

func (c *ClientImpl) Run(deployId string) (int, error) {
	req := &RunRequest{deployId}
	var reply RunReply
	err := c.client.Call("RpcServer.Run", req, &reply)
	if err != nil {
		return -1, err
	}

	// TODO return actual port?
	return -1, nil
}

func (c *ClientImpl) ListDeploys() ([]Deploy, error) {
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
