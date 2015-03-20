package main

import (
	"net/http"
	"os"
	"testing"
)

func waitForStatus(t *testing.T, r *Runner, status Status) {
	for {
		s := r.WaitForStatusChange()
		if s == status {
			if r.Status() != s {
				t.Fatalf("expected status of %v but was %v", s, r.Status())
			}
			return
		}
		// If we aren't specifically waiting for Error and we reach it, then
		// we'll never get to any other status.
		if s == Error {
			t.Fatalf("process reached error state while waiting for %v\n", status)
		}
	}
}

func TestRunner(t *testing.T) {
	r := NewRunner("testapp", "node app.js 8001", "/status", 8001)
	done := make(chan int)
	go func() {
		done <- r.RunLoop()
	}()
	waitForStatus(t, r, Running)
	resp, err := http.Get("http://localhost:8001")
	if err != nil {
		t.Fatalf("get: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health check failed")
	}
	r.Stop()
	retries := <-done
	if retries != 0 {
		t.Fatalf("Shouldn't have been any retries, but there were %d", retries)
	}
}

func killAndWaitForStopped(t *testing.T, r *Runner) {
	wait := make(chan int)
	go func() {
		waitForStatus(t, r, Stopped)
		wait <- 0
	}()
	proc, err := os.FindProcess(int(r.Pid))
	if err != nil {
		t.Fatalf("find process: %s\n", err)
	}
	if err = proc.Kill(); err != nil {
		t.Fatalf("Failed to kill process.")
	}
	<-wait
}

func TestRunnerRestartsCrash(t *testing.T) {
	r := NewRunner("testapp", "node app.js 8001", "/status", 8001)
	done := make(chan int)
	go func() {
		done <- r.RunLoop()
	}()
	waitForStatus(t, r, Running)
	killAndWaitForStopped(t, r)
	waitForStatus(t, r, Running)
	r.Stop()
	retries := <-done
	if retries < 1 {
		t.Fatalf("expected at least 1 retry, but there was %d\n", retries)
	}
	status := r.Status()
	if status != Stopped {
		t.Fatalf("expected status to be Stopped after RunLoop() returns, but was %v", status)
	}
}
