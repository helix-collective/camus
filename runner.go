package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Status int

const (
	// These states can lead to other states.
	Stopped Status = iota
	Running

	// These are exit states, the runner won't progress from them.
	FailedToStart
	ExitedInFailure
	ExitedCleanly
)

var statusName = []string{
	"STOPPED",
	"RUNNING",
	"FAILEDTOSTART",
	"EXITEDINFAILURE",
	"EXITEDCLEANLY",
}

func (s Status) String() string {
	return statusName[int(s)]
}

const (
	healthWaitDuartion time.Duration = 1 * time.Second

	maxRetries int = 5
)

type Runner struct {
	Dir        string
	Cmd        string
	HealthPath string
	Port       int
	stop       chan int
	Pid        int32

	cond   *sync.Cond
	lock   *sync.Mutex
	status Status
	logs   []string
}

func NewRunner(dir, cmd, healthPath string, port int) *Runner {
	lock := &sync.Mutex{}
	return &Runner{
		Dir:        dir,
		Cmd:        cmd,
		HealthPath: healthPath,
		Port:       port,
		stop:       make(chan int),
		lock:       lock,
		cond:       sync.NewCond(lock),
	}
}

func (r *Runner) checkHealth() bool {
	url := fmt.Sprintf("http://localhost:%d%s", r.Port, r.HealthPath)
	r.logf("Getting %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Failed to get: %s\n", err)
		return false
	}
	r.logf("Status code: %v\n", resp.StatusCode)
	return resp.StatusCode == http.StatusOK
}

func (r *Runner) setStatus(status Status) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.logfNolock("status change: %v -> %v\n", r.status, status)
	r.status = status
	r.cond.Signal()
}

func (r *Runner) Status() Status {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.status
}

func (r *Runner) logf(format string, args ...interface{}) {
	r.log(fmt.Sprintf(format, args...))
}

func (r *Runner) logfNolock(format string, args ...interface{}) {
	r.logNolock(fmt.Sprintf(format, args...))
}

func (r *Runner) log(msg string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.logNolock(msg)
}

func (r *Runner) logNolock(msg string) {
	r.logs = append(r.logs, msg)
}

func (r *Runner) Logs() []string {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.logs
}

func (r *Runner) Stop() {
	r.stop <- 0
}

// WaitForStatus waits for this process to acquire the given status, or return
// false if it never will.
func (r *Runner) WaitForStatus(status Status) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	for {
		if r.status == status || r.status > Running {
			return r.status == status
		}
		r.cond.Wait()
	}
}

func (r *Runner) run() bool {
	args := strings.Split(r.Cmd, " ")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = r.Dir
	r.logf("running %s\n", cmd.Args)
	err := cmd.Start()
	if err != nil {
		r.setStatus(FailedToStart)
		return false
	}
	atomic.StoreInt32(&r.Pid, int32(cmd.Process.Pid))
	healthOk := false
	for i := 0; i < maxRetries; i++ {
		r.logf("Checking health...\n")
		if r.checkHealth() {
			healthOk = true
			r.logf("Health is good!\n")
			break
		}
		time.Sleep(healthWaitDuartion)
	}
	if healthOk {
		r.setStatus(Running)
	} else {
		r.setStatus(FailedToStart)
	}
	exitChan := make(chan int)
	pid := cmd.Process.Pid
	go func() {
		exitState, err := cmd.Process.Wait()
		r.logf("process exited with status %v\n", exitState)
		if err != nil || !exitState.Success() {
			r.setStatus(ExitedInFailure)
		} else {
			r.setStatus(ExitedCleanly)
		}
		exitChan <- 0
	}()

	shouldLoop := true
	for {
		select {
		case <-exitChan:
			r.logf("Exiting, shouldLoop = %t\n", shouldLoop)
			// Process has exited.
			return shouldLoop
		case <-r.stop:
			r.logf("Killing process at callers request...\n")
			shouldLoop = false
			if p, err := os.FindProcess(pid); err == nil {
				p.Kill()
			}
		}
	}
}

func (r *Runner) RunLoop() int {
	retries := 0
	for r.run() {
		retries++
	}
	return retries
}

/*
type Store interface {
	Save(object interface{})
	Load(object interface{})
}

type State int

const (
	Stopped State = iota
	Loading
	Running

	// A runner is Blocked when it wants to listen on a given port, but the
	// port is already being listened to.
	Blocked
)

type RunInstruction struct {
	Port int
}

type Runner struct {
	deployId     string
	root         string
	port         int
	store        Store
	instructions <-chan interface{}

	app Application

	// Protects state.
	lock  *sync.Mutex
	state State
}

func (r *Runner) setStatus(state State) {
	r.lock.Lock()
	r.state = state
	r.lock.Unlock()
}

func (r *Runner) getState() State {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.state
}

func (r *Runner) Run(deployId, cmd, healthEndpoint string, port int) {
	go r.run(deployId, cmd, healthEndpoint, port)
}

func (r *Runner) run(deployId, cmd, healthEndpoint string, port int) {
	for {
		procs := FindListeningProcesses(port, port)
		if len(procs) > 0 {
			// Something is already listening on this port.
			r.setStatus(Blocked)
		}
		time.Sleep(1 * time.Second)
	}
	r.setStatus(Loading)
	workingDir := path.Join(r.root, deploysDirName, deployId)


}
*/
