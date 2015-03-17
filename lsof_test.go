package main

import (
	"strings"
	"testing"
)

var dummyOutput = strings.Trim(`
COMMAND   PID           USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node    53530 jameskozianski   12u  IPv4 0xccadfbbeb791d7fb      0t0  TCP *:8001 (LISTEN)
node    57065 jameskozianski   14u  IPv4 0xccadfbbec7e5c7fb      0t0  TCP *:8002 (LISTEN)
`, "\n")

var dummyCwdLookupOutput = strings.Trim(`
COMMAND   PID           USER   FD   TYPE DEVICE SIZE/OFF     NODE NAME
node    25975 jameskozianski  cwd    DIR    1,4       68 29604824 /var/testapp/a b c d
`, "\n")

var dummyCwdLookupOutputNoSpaces = strings.Trim(`
COMMAND   PID           USER   FD   TYPE DEVICE SIZE/OFF     NODE NAME
node    24463 jameskozianski  cwd    DIR    1,4      170 29492629 /var/deploys/2015-03-15-14-43-08
`, "\n")

func TestLookupPorts(t *testing.T) {
	procs, err := parseLookupPortOutput(dummyOutput)
	if err != nil {
		t.Fail()
	}
	found := false
	for _, proc := range procs {
		if proc.Port == 8001 {
			found = true
			if proc.Name != "node" {
				t.Error("program listening on 8001 should be node")
			}
		}
	}
	if !found {
		t.Error("Failed to find node listening on 8001")
	}
}

func TestLookupCwd(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{dummyCwdLookupOutput, "/var/testapp/a b c d"},
		{dummyCwdLookupOutputNoSpaces, "/var/deploys/2015-03-15-14-43-08"},
	}
	for _, test := range tests {
		cwd, err := parseLookupCwdOutput(test.input)
		if err != nil {
			t.Fatalf("failed to parse lsof output")
		}
		if cwd != test.expected {
			t.Errorf("cwd is '%s', expected '%s'\n", cwd, test.expected)
		}
	}
}
