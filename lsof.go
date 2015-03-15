package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type Process struct {
	Port int
	Name string
	Pid  int
}

func Lsof() ([]*Process, error) {
	cmd := exec.Command("lsof", "-i", "-P", "-M")
	data, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseLsofOutput(string(data))
}

func parseLsofOutput(str string) ([]*Process, error) {
	lines := strings.Split(str, "\n")[1:]
	processes := []*Process{}
	for _, line := range lines {
		if line == "" {
			continue
		}
		proc := parseProcess(line)
		if proc != nil {
			processes = append(processes, proc)
		}
	}
	return processes, nil
}

func parseProcess(line string) *Process {
	spaces := regexp.MustCompile(" +")
	words := spaces.Split(line, -1)
	name := words[0]
	pid, err := strconv.Atoi(words[1])
	if err != nil {
		fmt.Printf("Failed to parse line from lsof, ignoring...\n%s\n", words[1])
		return nil
	}
	portWord := words[8]
	re := regexp.MustCompile("^.*:(.*)$")
	port, err := strconv.Atoi(string(re.FindSubmatch([]byte(portWord))[1]))
	if err != nil {
		return nil
	}
	// We only care about processes that are listening.
	if len(words) < 10 || words[9] != "(LISTEN)" {
		return nil
	}
	return &Process{
		Port: port,
		Name: name,
		Pid:  pid,
	}
}
