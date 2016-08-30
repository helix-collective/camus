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
	"strings"
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

	// The id of the process running this deploy.
	Pid int

	// http status code,
	// 0 for nothing running on port (or no port specified)
	// negative timeout or something else wrong with the deploy
	//
	// if 0, and port is specified, then it's safe to run the binary
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
	haproxyConfig        = "haproxy.cfg"
	haproxyPid           = "haproxy.pid"
	appPid               = "PID_FILE"
	minShortNameLength   = 3
)

type Config struct {
	Ports map[int]string
}

type ServerImpl struct {
	root         string
	config       Config
	startPort    int
	endPort      int
	client       *http.Client
	deploysPath  string
	enforceDelay time.Duration
}

func readConfig(path string) (Config, error) {
	config := Config{
		Ports: map[int]string{},
	}
	if data, err := ioutil.ReadFile(path); err == nil {
		c := struct {
			Ports map[string]string
		}{}
		err = json.Unmarshal(data, &c)
		if err != nil {
			return Config{}, err
		}
		for portStr, deployId := range c.Ports {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return Config{}, fmt.Errorf("Ports keys should be numbers")
			}
			config.Ports[port] = deployId
		}
	}
	return config, nil
}

func NewServerImpl(
	root string,
	autoEnforce bool,
	portBase int) (*ServerImpl, error) {

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
		os.MkdirAll(deploysPath, 0744)
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New("health check should not redirect")
		},
		Timeout: MAX_HEALTH_CHECK_TIME,
	}

	server := &ServerImpl{
		root:         root,
		config:       config,
		startPort:    portBase + 1,
		endPort:      portBase + 99,
		client:       client,
		deploysPath:  deploysPath,
		enforceDelay: time.Duration(5) * time.Second,
	}

	if autoEnforce {
		go server.EnforceLoop()
	}

	return server, nil
}

func (s *ServerImpl) NewDeployDir() NewDeployDirResponse {
	deployId := NewDeployId()

	return NewDeployDirResponse{
		DeployId: deployId,
		Path:     s.deployDir(deployId),
	}
}

func (s *ServerImpl) DeploysPath() string {
	return s.deploysPath
}
func (s *ServerImpl) deployDir(deployId string) string {
	return path.Join(s.deploysPath, deployId)
}
func (s *ServerImpl) deployConfigFile(deployId string) string {
	return path.Join(s.deployDir(deployId), deployConfigFileName)
}

func (s *ServerImpl) EnforceLoop() {
	for {
		s.Enforce()
		time.Sleep(s.enforceDelay)
	}
}

func (s *ServerImpl) Enforce() {
	procs := FindListeningProcesses(s.startPort, s.endPort)
	procsByPort := makeProcessPortLookup(procs)
	for port, deployId := range s.config.Ports {
		// deployId should be running on port.
		running, ok := procsByPort[port]
		if !ok {
			// Nothing is running on port, so we should run our deploy.
			// TODO(koz): Wait for health of all started deploys in parallel.
			s.startDeployAndWaitForHealth(deployId, port)
			continue
		}

		//process is running, now check it's the *right* one
		//if the deploy has written to a pid file then trust this app-specific override
		if pid, err := s.getDeployPidOverride(deployId); err != nil {
			fmt.Printf("warning: could not read deploy %s's pid file: %s\n", deployId, err)
		} else if pid < 0 {
			//no pid override
		} else if running.Pid == pid {
			continue
		}

		//otherwise, check the default: the deployed app in the deploy dir on the deploy port
		if running.DeployId != deployId {
			runningDeploy := running.DeployId
			if runningDeploy == "" {
				runningDeploy = fmt.Sprintf("(pid:%d)", running.Pid)
			}
			// Something unexpected is running on port, so report it.
			fmt.Printf("%s, not %s is running on %d\n", runningDeploy, deployId, port)
			continue
		}
	}
}

func (s *ServerImpl) getDeployPidOverride(deployId string) (int, error) {
	pidFile := path.Join(s.deployDir(deployId), appPid)
	if _, statErr := os.Stat(pidFile); statErr != nil {
		return -1, nil //file doesn't exist = no pid override
	} else if pid, err := readPid(pidFile); err != nil {
		return -1, errors.New(fmt.Sprintf("warning: could not read deploy %s's pid file: %s", deployId, err))
	} else {
		return pid, nil
	}
}

func makeProcessPortLookup(procs []Process) map[int]Process {
	result := map[int]Process{}
	for _, proc := range procs {
		result[proc.Port] = proc
	}
	return result
}

func makeProcessPidLookup(procs []Process) map[int]Process {
	result := map[int]Process{}
	for _, proc := range procs {
		result[proc.Pid] = proc
	}
	return result
}

func makeProcessDeployIdLookup(procs []Process) map[string]Process {
	result := map[string]Process{}
	for _, proc := range procs {
		result[proc.DeployId] = proc
	}
	return result
}

func (s *ServerImpl) startDeployAndWaitForHealth(deployId string, port int) error {
	app, cmd, err := s.commandForDeploy(deployId, port)
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := s.waitForAppToStart(port, app); err != nil {
		return err
	}
	return nil
}

func (s *ServerImpl) readDeployIdsFromDisk() []string {
	infos, err := ioutil.ReadDir(s.deploysPath)
	if err != nil {
		panic(fmt.Errorf("read deploy dir: %s", err))
	}
	var result []string
	for _, info := range infos {
		result = append(result, info.Name())
	}
	return result
}

func (s *ServerImpl) checkAllHealth(deploys []*Deploy) {
	healthChecks := 0
	checkSync := make(chan int)
	for _, deploy := range deploys {
		healthChecks++
		go func(deploy *Deploy) {
			s.checkHealth(deploy)
			checkSync <- 0
		}(deploy)
	}

	for healthChecks > 0 {
		<-checkSync
		healthChecks--
	}
}

func (s *ServerImpl) ListDeploys() ([]*Deploy, error) {
	procs := FindListeningProcesses(s.startPort, s.endPort)
	procsByDeployId := makeProcessDeployIdLookup(procs)
	procsByPid := makeProcessPidLookup(procs)
	unaccountedProcsByPort := makeProcessPortLookup(procs)
	knownRunningDeploys := []*Deploy{}
	deployIds := s.readDeployIdsFromDisk()
	knownDeploys := []*Deploy{}
	for _, deployId := range deployIds {
		proc, running := procsByDeployId[deployId]
		if pidOverride, err := s.getDeployPidOverride(deployId); err == nil {
			if _, ok := procsByPid[pidOverride]; ok {
				//found a process matching the app pid override
				proc, running = procsByPid[pidOverride]
			}
		}
		deploy := &Deploy{
			Id:      deployId,
			Pid:     proc.Pid,
			Port:    proc.Port,
			Tracked: s.lookupConfiguredPort(deployId) != 0,
		}
		if running {
			delete(unaccountedProcsByPort, proc.Port)
			knownRunningDeploys = append(knownRunningDeploys, deploy)
		} else {
			deploy.Port = s.lookupConfiguredPort(deployId)
		}
		knownDeploys = append(knownDeploys, deploy)
	}
	// Any processes that haven't been accounted for yet, we list them as deploys, too.
	unaccounted := []*Deploy{}
	for _, proc := range unaccountedProcsByPort {
		unaccounted = append(unaccounted, &Deploy{
			Id:      fmt.Sprintf("%s-%d", proc.Name, proc.Port),
			Pid:     proc.Pid,
			Port:    proc.Port,
			Tracked: false,
		})
	}
	s.checkAllHealth(knownRunningDeploys)
	return append(knownDeploys, unaccounted...), nil
}

func (s *ServerImpl) checkHealth(deploy *Deploy) {
	app, err := ApplicationFromConfig(false, s.deployConfigFile(deploy.Id))
	if err != nil {
		deploy.Errors = append(deploy.Errors,
			fmt.Sprintf("Missing deploy config (%s)", err))
		println("Missing config")
		deploy.Health = -2
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

func (s *ServerImpl) findUnusedPort() (int, error) {
	for i := s.startPort; i <= s.endPort; i++ {
		if !s.portConfigured(i) && portFree(i) {
			return i, nil
		}
	}
	return -1, errors.New("Could not find free port")
}

// lookupConfiguredPort returns the port the specified deploy is configured to
// run on, or 0 if it's not configured to run anywhere.
func (s *ServerImpl) lookupConfiguredPort(deployId string) int {
	for port, id := range s.config.Ports {
		if id == deployId {
			return port
		}
	}
	return 0
}

func (s *ServerImpl) portConfigured(port int) bool {
	_, taken := s.config.Ports[port]
	return taken
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
	c := struct {
		Ports map[string]string
	}{
		Ports: map[string]string{},
	}
	for port, deployId := range s.config.Ports {
		c.Ports[strconv.Itoa(port)] = deployId
	}
	data, err := json.MarshalIndent(&c, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(s.root, serverConfigFileName),
		data, os.FileMode(0644))
}

func (s *ServerImpl) SetActiveByPort(port int) error {
	return s.reloadHaproxy(port)
}

func (s *ServerImpl) SetActiveById(id string) error {
	for port, deployId := range s.config.Ports {
		if deployId == id {
			return s.reloadHaproxy(port)
		}
	}

	return fmt.Errorf("No deploy %s, run 'list' to see valid deploys", id)
}

func (s *ServerImpl) GetFullDeployIdFromShortName(deployShortName string) (string, error) {
	if len(deployShortName) < minShortNameLength {
		return "", fmt.Errorf("Deploy name substring is too short, needs to be at least %d characters", minShortNameLength)
	}

	var matchingIds []string
	for _, deployId := range s.readDeployIdsFromDisk() {
		if strings.Contains(deployId, deployShortName) {
			matchingIds = append(matchingIds, deployId)
		}
	}

	numMatching := len(matchingIds)
	if numMatching != 1 {
		return "", fmt.Errorf("short name corresponds to %d deploy IDs, be more specific", numMatching)
	}

	return matchingIds[0], nil
}

func (s *ServerImpl) Run(deployIdToRun string) (int, error) {
	for port, deployId := range s.config.Ports {
		if deployIdToRun == deployId {
			return -1, fmt.Errorf("Already configured for port %d", port)
		}
	}

	port, err := s.findUnusedPort()
	if err != nil {
		return -1, err
	}

	app, cmd, err := s.commandForDeploy(deployIdToRun, port)
	if err != nil {
		return -1, err
	}

	s.config.Ports[port] = deployIdToRun
	err = s.writeConfig()
	if err != nil {
		return -1, fmt.Errorf("write config: %s", err)
	}

	err = cmd.Start()
	if err != nil {
		return -1, err
	}

	if err := s.waitForAppToStart(port, app); err != nil {
		return -1, err
	}

	return port, nil
}

func (s *ServerImpl) Stop(deployIdToStop string) error {
	procs := FindListeningProcesses(s.startPort, s.endPort)
	procsByDeployId := makeProcessDeployIdLookup(procs)
	proc, running := procsByDeployId[deployIdToStop]
	port := s.lookupConfiguredPort(deployIdToStop)
	if port == 0 {
		return fmt.Errorf("Deploy not running or not on a port")
	}

	delete(s.config.Ports, port)
	err := s.writeConfig()
	if err != nil {
		return fmt.Errorf("write config: %s", err)
	}

	//kill the proc *after* removing it from the list so it doesn't auto-restart
	if running {
		if p, err := os.FindProcess(proc.Pid); err == nil {
			//try to kill by process group id so the whole bundle incl. children gets cleaned up
			if pgid, pgerr := syscall.Getpgid(proc.Pid); pgerr == nil {
				syscall.Kill(-pgid, 15) //minus is required
			} else {
				p.Kill()
			}
		} else {
			return fmt.Errorf("Failed to kill pid %s", proc.Pid)
		}
	} else {
		return fmt.Errorf("Deploy not running")
	}
	return nil
}

func (s *ServerImpl) commandForDeploy(deployIdToRun string, port int) (Application, *exec.Cmd, error) {
	deployPath := s.deployDir(deployIdToRun)
	app, err := ApplicationFromConfig(false,
		path.Join(deployPath, "deploy.json"))

	if err != nil {
		return nil, nil, err
	}
	cmd := exec.Command("sh", "-c", app.RunCmd(port))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = deployPath
	detachProc(cmd)
	return app, cmd, nil
}

func (s *ServerImpl) CleanupDeploy(deployIdToCleanup string) error {
	procs := FindListeningProcesses(s.startPort, s.endPort)
	procsByDeployId := makeProcessDeployIdLookup(procs)
	_, running := procsByDeployId[deployIdToCleanup]
	if running {
		return errors.New("Cannot cleanup. Deploy is currently running.")
	}

	configPort := s.lookupConfiguredPort(deployIdToCleanup)
	delete(s.config.Ports, configPort)
	err := s.writeConfig()
	if err != nil {
		return fmt.Errorf("write config: %s", err)
	}

	pathToCleanUp := s.deployDir(deployIdToCleanup)
	if _, err := os.Stat(pathToCleanUp); err != nil {
		return err
	}
	return os.RemoveAll(pathToCleanUp)
}

func detachProc(cmd *exec.Cmd) {
	// give it its own process group, so it doesn't die
	// when the manager process exits for whatever reason
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

var MAX_STARTUP_TIME = time.Duration(20) * time.Second
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

func (s *ServerImpl) reloadHaproxy(port int) error {
	if port < s.startPort {
		return fmt.Errorf("Invalid prod port %d", port)
	}
	cfg := HaproxyConfig(s.endPort, s.endPort-1, port)

	cfgFile := path.Join(s.root, haproxyConfig)
	pidFile := path.Join(s.root, haproxyPid)

	if err := ioutil.WriteFile(cfgFile, []byte(cfg), os.FileMode(0644)); err != nil {
		return err
	}

	runningPid, err := readPid(pidFile)
	if err != nil {
		return err
	}

	cmd := haproxyCmd(cfgFile, pidFile, runningPid)

	return cmd.Run()
}

func haproxyCmd(cfgFile string, pidFile string, runningPid int) *exec.Cmd {
	log.Println("PID ", runningPid, " ", pidFile)
	var cmd *exec.Cmd
	if runningPid > 0 {
		cmd = exec.Command(
			"haproxy",
			"-f", cfgFile,
			"-p", pidFile,
			"-sf", strconv.Itoa(runningPid))
	} else {
		cmd = exec.Command(
			"haproxy",
			"-f", cfgFile,
			"-p", pidFile)
	}

	detachProc(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}

func readPid(pidFile string) (int, error) {
	if data, err := ioutil.ReadFile(pidFile); err == nil {
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return -1, fmt.Errorf("Invalid pid data, %s", err)
		}
		return pid, nil
	} else {
		return -1, nil // OK - no current pid
	}

}

func contains(strs []string, str string) bool {
	for _, s := range strs {
		if s == str {
			return true
		}
	}
	return false
}

// TODO(koz): Don't return haproxy processes here.
func (s *ServerImpl) findUnknownProcesses() []Process {
	procs := FindListeningProcesses(s.startPort, s.endPort)
	deployIds := s.readDeployIdsFromDisk()
	unknown := []Process{}
	for _, proc := range procs {
		if !contains(deployIds, proc.DeployId) {
			unknown = append(unknown, proc)
		}
	}
	return unknown
}

func (s *ServerImpl) KillUnknownProcesses() {
	for _, proc := range s.findUnknownProcesses() {
		if p, err := os.FindProcess(proc.Pid); err == nil {
			p.Kill()
		}
	}
}

// Shutdown kills all processes in the range of camus and then exits.
func (s *ServerImpl) Shutdown() {
	procs := FindListeningProcesses(s.startPort, s.endPort)
	for _, proc := range procs {
		if p, err := os.FindProcess(proc.Pid); err == nil {
			p.Kill()
		}
	}
	os.Exit(0)
}
