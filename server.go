package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

type Deploy struct {
	Id   string
	Note string

	// known to the system config, versus just found either
	// running on a port or in the deploys dir
	Tracked bool

	// Port either specified in the config (regardless of running)
	// or port running on, for untracked things found on a port.
	// -1 for not specified
	Port int

	// http status code,
	// 0 for no connection attempted,
	// -1 for connection failed or timeout
	Health int

	Errors []string
}

type Label string

type Server interface {
	ListLabels() ([]Label, error)
	ListDeploys() ([]*Deploy, error)
	Run(deployId string) error
	Stop(deployId string) error
	Label(deployId string, label Label) error

	// TODO Maintenance mode
}

const (
	deploysDirName       = "deploys"
	deployConfigFileName = "deploy.json"
	serverConfigFileName = "config.json"
)

type Config struct {
	Ports  map[string]string
	Labels map[string]string
}

type ServerImpl struct {
	root        string
	config      *Config
	startPort   int
	endPort     int
	client      *http.Client
	deploysPath string
}

func readConfig(path string) (*Config, error) {
	var config Config
	if data, err := ioutil.ReadFile(path); err == nil {
		err = json.Unmarshal(data, &config)
		if err != nil {
			return nil, err
		}
	}
	if config.Ports == nil {
		config.Ports = make(map[string]string)
	}
	if config.Labels == nil {
		config.Labels = make(map[string]string)
	}
	return &config, nil
}

func NewServerImpl(root string) (*ServerImpl, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		log.Fatal("Root path:", err)
	}
	config, err := readConfig(path.Join(root, serverConfigFileName))
	if err != nil {
		return nil, err
	}
	deploysPath := path.Join(root, deploysDirName)
	if _, err = os.Open(deploysPath); os.IsNotExist(err) {
		os.MkdirAll(deploysPath, 0644)
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New("health check should not redirect")
		},
		Timeout: MAX_HEALTH_CHECK_TIME,
	}
	return &ServerImpl{
		root:        root,
		config:      config,
		startPort:   8001,
		endPort:     8099,
		client:      client,
		deploysPath: deploysPath,
	}, nil
}

func (s *ServerImpl) NewDeployDir() NewDeployDirResponse {
	t := time.Now()
	timestamp := fmt.Sprintf("%d-%02d-%02d-%02d-%02d-%02d",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())

	return NewDeployDirResponse{
		DeployId: timestamp,
		Path:     s.deployDir(timestamp),
	}
}

func (s *ServerImpl) deployDir(deployId string) string {
	return path.Join(s.deploysPath, deployId)
}
func (s *ServerImpl) deployConfigFile(deployId string) string {
	return path.Join(s.deployDir(deployId), deployConfigFileName)
}

func (s *ServerImpl) ListDeploys() ([]*Deploy, error) {

	result, err := s.scanDeployDirs()
	if err != nil {
		return nil, err
	}

	result = s.scanConfig(result)
	result = s.scanPorts(result)

	return result, nil
}

func (s *ServerImpl) scanDeployDirs() ([]*Deploy, error) {
	infos, err := ioutil.ReadDir(s.deploysPath)
	if err != nil {
		return nil, err
	}

	var result []*Deploy
	for _, info := range infos {
		result = append(result, &Deploy{
			Id:      info.Name(),
			Port:    -1,
			Tracked: false,
		})
	}

	return result, nil
}

func (s *ServerImpl) scanConfig(deploys []*Deploy) []*Deploy {
	result := []*Deploy{}
	result = append(result, deploys...)

	for portStr, deployId := range s.config.Ports {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			println(err)
			continue
		}

		deploy := findDeployById(deployId, result)

		if deploy == nil {
			result = append(result, &Deploy{
				Id:      deployId,
				Port:    port,
				Tracked: true,
				Errors:  []string{"No deploy dir present!"},
			})
		} else {
			deploy.Port = port
			deploy.Tracked = true
		}
	}

	return result
}

// Finds ports with *something* on them,
// either checking the health status for known deploys
// (and updating its run state)
// or adding a deploy with "unknown" id.
func (s *ServerImpl) scanPorts(deploys []*Deploy) []*Deploy {
	healthChecks := 0
	checkSync := make(chan int)

	result := []*Deploy{}
	result = append(result, deploys...)

	for port := s.startPort; port <= s.endPort; port++ {
		if portFree(port) {
			continue
		}

		dep := findDeployByPort(port, result)
		if dep == nil {
			result = append(result, &Deploy{
				Id:      fmt.Sprintf("(unkonwn-%d)", port),
				Port:    port,
				Tracked: false,
				Health:  0,
			})
		} else {
			dep.Tracked = true
			healthChecks++
			go func(deploy *Deploy) {
				s.checkHealth(deploy)
				checkSync <- 0
			}(dep)
		}
	}

	for healthChecks > 0 {
		<-checkSync
		healthChecks--
	}

	return result
}

func (s *ServerImpl) checkHealth(deploy *Deploy) {
	app, err := ApplicationFromConfig(s.deployConfigFile(deploy.Id))
	if err != nil {
		deploy.Errors = append(deploy.Errors,
			fmt.Sprintf("!! missing deploy config !! %s", err))
		println("Missing config")
		deploy.Health = -4
		return
	}

	status, err := s.testApp(deploy.Port, app)
	if err != nil {
		deploy.Errors = append(deploy.Errors, fmt.Sprintf("%s", err))
		log.Println("Got http err ", err, " for ", deploy.Id)
		deploy.Health = -1
		return
	}

	deploy.Health = status
}

func findDeployByPort(port int, deploys []*Deploy) *Deploy {
	for _, deploy := range deploys {
		if deploy.Port == port {
			return deploy
		}
	}
	return nil
}

func findDeployById(id string, deploys []*Deploy) *Deploy {
	for _, deploy := range deploys {
		if deploy.Id == id {
			return deploy
		}
	}
	return nil
}

func (s *ServerImpl) findUnusedPort() (int, error) {
	for i := s.startPort; i <= s.endPort; i++ {

		if portFree(i) {
			return i, nil
		}

	}

	return -1, errors.New("Could not find free port")
}

func portFree(port int) bool {
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		// TODO: Is this now safe to assume the port is free?
		// NOTE(dan): I tried implementing listening on the port
		// instead, but it always succeeded even if there was
		// actually something already there...
		return true
	} else {
		conn.Close()
		return false
	}
}

func (s *ServerImpl) writeConfig() error {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(s.root, serverConfigFileName),
		data, os.FileMode(0644))
}

func (s *ServerImpl) Run(deployId string) error {
	port, err := s.findUnusedPort()
	if err != nil {
		return err
	}
	log.Println("Found port ", port)

	deployPath := s.deployDir(deployId)

	app, err := ApplicationFromConfig(path.Join(deployPath, "deploy.json"))
	if err != nil {
		return err
	}

	s.config.Ports[strconv.Itoa(port)] = deployId
	err = s.writeConfig()
	if err != nil {
		return fmt.Errorf("write config: %s", err)
	}
	println(deployPath)
	println(app.RunCmd(port))
	cmd := exec.Command("sh", "-c", app.RunCmd(port))

	// process working dir
	cmd.Dir = deployPath

	// give it its own process group, so it doesn't die
	// when the manager process exits for whatever reason
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Setpgid = true

	err = cmd.Start()
	if err != nil {
		return err
	}

	return s.waitForAppToStart(port, app)
}

var MAX_STARTUP_TIME = time.Duration(10) * time.Second
var MAX_HEALTH_CHECK_TIME = time.Duration(2) * time.Second
var STARTUP_HEALTH_CHECK_INTERVAL = time.Duration(100) * time.Millisecond

func (s *ServerImpl) waitForAppToStart(port int, app Application) error {

	end := time.Now().Add(MAX_STARTUP_TIME)
	for {
		log.Print(".")

		status, err := s.testApp(port, app)

		if err == nil {
			if status == 200 {
				log.Println("ok")
				return nil
			} else {
				log.Println("bad:", status)
				return errors.New(fmt.Sprintf("Health check failed %d", status))
			}
		}

		if time.Now().After(end) {
			return errors.New("Failed to connect to app after timeout")
		}

		time.Sleep(STARTUP_HEALTH_CHECK_INTERVAL)
	}
}

func (s *ServerImpl) testApp(port int, app Application) (int, error) {
	resp, err := s.client.Get(
		fmt.Sprintf("http://localhost:%d%s", port, app.HealthEndpoint()))

	if err == nil {
		return resp.StatusCode, nil
	}

	return -1, err
}
