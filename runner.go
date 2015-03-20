package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type Status int

const (
	// The task isn't running.
	Stopped Status = iota

	// The task is starting up, but hasn't become healthy yet.
	Starting

	// The task is currently running.
	Running

	// Unrecoverable error.
	Error
)

var statusName = []string{
	"Stopped",
	"Starting",
	"Running",
	"Error",
}

func (s Status) String() string {
	return statusName[int(s)]
}

const (
	healthWaitDuration time.Duration = 1 * time.Second

	maxRunnerRetries int = 5
)

type Runner struct {
	Dir        string
	Cmd        string
	HealthPath string
	Port       int
	stop       chan int
	Pid        int32

	// cond is a condition variable on status changing, with lock as its
	// lockable. lock guards both status and logs.
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
	r.cond.Broadcast()
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

func (r *Runner) WaitForStatusChange() Status {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.cond.Wait()
	return r.status
}

func (r *Runner) run() bool {
	cmd := exec.Command("sh", "-c", r.Cmd)
	cmd.Dir = r.Dir
	r.logf("running %s\n", cmd.Args)
	err := cmd.Start()
	if err != nil {
		r.logf("Failed to start\n")
		r.setStatus(Error)
		return false
	}
	r.setStatus(Starting)
	atomic.StoreInt32(&r.Pid, int32(cmd.Process.Pid))
	// TODO(koz): Separate the health checking into a separate goroutine /
	// state variable.
	healthOk := false
	for i := 0; i < maxRunnerRetries; i++ {
		r.logf("Checking health...\n")
		if r.checkHealth() {
			healthOk = true
			r.logf("Health is good!\n")
			break
		}
		time.Sleep(healthWaitDuration)
	}
	if healthOk {
		r.setStatus(Running)
	} else {
		// Health check failed at startup = unrecoverable error.
		r.setStatus(Error)
	}
	exitChan := make(chan int)
	pid := cmd.Process.Pid
	go func() {
		exitState, _ := cmd.Process.Wait()
		r.logf("process exited with status %v\n", exitState)
		r.setStatus(Stopped)
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
			// We don't use cmd.Process.Kill() here as Process is used in the
			// goroutine that's Wait()ing on the process to terminate.
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
