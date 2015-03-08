package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"
)

type Server interface {
	ListLabels() ([]Label, error)
	ListDeploys() ([]Deploy, error)
	Run(deployId string) error
	Stop(deployId string) error
	Label(deployId string, label Label) error

	// TODO Maintenance mode
}

const (
	deployPath = "deploys"
	configPath = "config.json"
)

type Config struct {
	Ports  map[int]string
	Labels map[string]string
}

type ServerImpl struct {
	root   string
	config Config
}

func NewServerImpl(root string) *ServerImpl {
	root, err := filepath.Abs(root)
	if err != nil {
		log.Fatal("Root path:", err)
	}

	cfgPath := path.Join(root, configPath)
	data, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		log.Fatal("ReadFile:", err)
	}
	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatal("Unmarshal:", err)
	}
	return &ServerImpl{root, config}
}

func (s *ServerImpl) NewDeployDir() string {
	// TODO(koz): Use a timestamp.

	t := time.Now()
	timestamp := fmt.Sprintf("%d-%02d-%02d-%02d-%02d-%02d",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())

	return path.Join(s.root, deployPath, timestamp)
}

func (s *ServerImpl) ListDeploys() ([]Deploy, error) {
	infos, err := ioutil.ReadDir(path.Join(s.root, deployPath))
	if err != nil {
		return nil, err
	}
	var result []Deploy
	for _, info := range infos {
		result = append(result, Deploy{
			Id:   info.Name(),
			Port: -1,
		})
	}
	return result, nil
}

func (s *ServerImpl) findUnusedPort() (int, error) {
	// TODO(koz): Implement.
	return 8100, nil
}

func (s *ServerImpl) writeConfig() error {
	data, err := json.Marshal(s.config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(s.root, configPath), data, os.FileMode(0644))
}

func (s *ServerImpl) Run(deployId string) error {
	port, err := s.findUnusedPort()
	if err != nil {
		return err
	}

	s.config.Ports[port] = deployId
	s.writeConfig()
	// TODO(koz): Don't hardcode testapp values.
	cmd := exec.Command(fmt.Sprintf("node app.js %s", port))
	cmd.Path = path.Join(s.root, deployPath, deployId)
	err = cmd.Start()
	if err != nil {
		log.Fatal("exec:", err)
	}
	return nil
}

type RpcServer struct {
	server *ServerImpl
}

type ListDeploysRequest struct{}
type ListDeploysReply struct {
	Deploys []Deploy
}

func (s *RpcServer) ListDeploys(arg ListDeploysRequest, reply *ListDeploysReply) error {
	deploys, err := s.server.ListDeploys()
	if err != nil {
		return err
	}
	reply.Deploys = deploys
	return nil
}

type RunRequest struct {
	DeployId string
}

type RunReply struct {
}

func (s *RpcServer) Run(arg RunRequest, reply *RunReply) error {
	return s.server.Run(arg.DeployId)
}

type NewDeployDirRequest struct {
}

type NewDeployDirResponse struct {
	Path string
}

func (s *RpcServer) NewDeployDir(arg NewDeployDirRequest, reply *NewDeployDirResponse) error {
	reply.Path = s.server.NewDeployDir()
	return nil
}
