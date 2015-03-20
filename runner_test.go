package main

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestRunner(t *testing.T) {
	r := NewRunner("testapp", "node app.js 8001", "/status", 8001)
	done := make(chan int)
	go func() {
		r.RunLoop()
		done <- 0
	}()
	if !r.WaitForStatus(Running) {
		t.Fatalf("Server failed to get healthy")
	}
	resp, err := http.Get("http://localhost:8001")
	if err != nil {
		t.Fatalf("get: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health check failed")
	}
	r.Stop()
	<-done
	if r.WaitForStatus(Running) {
		t.Fatalf("Shouldn't succeed waiting on a dead process")
	}
}

func TestRunnerRestartsCrash(t *testing.T) {
	r := NewRunner("testapp", "node app.js 8001", "/status", 8001)
	done := make(chan int)
	go func() {
		done <- r.RunLoop()
	}()
	if !r.WaitForStatus(Running) {
		t.Fatalf("Server failed to get healthy")
	}
	proc, err := os.FindProcess(int(r.Pid))
	if err != nil {
		t.Fatalf("find process: %s\n", err)
	}
	if err = proc.Kill(); err != nil {
		t.Fatalf("Failed to kill process.")
	}
	time.Sleep(1 * time.Second)
	if !r.WaitForStatus(Running) {
		t.Fatalf("Server failed to recover from being killed")
	}
	r.Stop()
	retries := <-done
	if retries < 1 {
		t.Fatalf("expected at least 1 retry, but there was %d\n", retries)
	}
	if r.WaitForStatus(Running) {
		t.Fatalf("Shouldn't succeed waiting on a dead process")
	}
}
