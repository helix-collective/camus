package main

import (
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type Process struct {
	Port     int
	Name     string
	Pid      int
	DeployId string
}

// FindListeningProcesses returns a list of Processes that are listening on
// ports between lowPort and highPort. It will try and use the current working
// directory to determine which deployId the running process has.
// TODO(koz): Provide a fake shell so we can test this function.
func FindListeningProcesses(lowPort, highPort int) []Process {
	// List all TCP sockets listening between lowPort and highPort (-P prevents
	// trying to resolve port numbers to well-known service names).
	portRange := fmt.Sprintf(":%d-%d", lowPort, highPort)
	cmd := exec.Command("lsof", "-P", "-i", portRange, "-sTCP:LISTEN")
	// The command will fail if there are no matching processes, so ignore the error.
	data, _ := cmd.Output()
	procs, err := parseLookupPortOutput(string(data))
	if err != nil {
		panic(err)
	}
	for i := range procs {
		cwd, err := lookupCwd(procs[i].Pid)
		if err != nil {
			fmt.Printf("Failed to lookup cwd for %d: %s\n", procs[i].Pid, err)
			continue
		}
		procs[i].DeployId = deriveDeployIdFromCwd(cwd)
	}
	return procs
}

func deriveDeployIdFromCwd(cwd string) string {
	dir, deployId := path.Split(cwd)
	dir, deploysDir := path.Split(strings.TrimSuffix(dir, "/"))
	// Check to see if this process is one of ours.
	// TODO(koz): Make this strict by passing in the server root.
	if deploysDir != "deploys" {
		return ""
	}
	return deployId
}

func parseLookupCwdOutput(str string) (string, error) {
	lines := strings.Split(str, "\n")
	header := lines[0]
	fields := lines[1]
	i := strings.Index(header, "NAME")
	if i == -1 {
		return "", fmt.Errorf("Failed to parse lsof output, expected NAME in header")
	}
	return fields[i:], nil
}

func lookupCwd(pid int) (string, error) {
	// Get current working directory of given pid.
	cmd := exec.Command("lsof", "-a", "-d", "cwd", "-p", strconv.Itoa(pid))
	data, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return parseLookupCwdOutput(string(data))
}

func parseLookupPortOutput(str string) ([]Process, error) {
	lines := strings.Split(str, "\n")[1:]
	processes := []Process{}
	for _, line := range lines {
		if line == "" {
			continue
		}
		proc, err := parseProcess(line)
		if err == nil {
			processes = append(processes, proc)
		}
	}
	return processes, nil
}

func parseProcess(line string) (Process, error) {
	words := regexp.MustCompile(" +").Split(line, -1)
	name := words[0]
	pid, err := strconv.Atoi(words[1])
	if err != nil {
		fmt.Printf("Failed to parse line from lsof, ignoring...\n%s\n", words[1])
		return Process{}, err
	}
	portWord := words[8]
	re := regexp.MustCompile("^.*:(.*)$")
	port, err := strconv.Atoi(string(re.FindSubmatch([]byte(portWord))[1]))
	if err != nil {
		return Process{}, err
	}
	return Process{
		Port: port,
		Name: name,
		Pid:  pid,
	}, nil
}

/*
func main() {
	ps, err := FindListeningProcesses(8000, 8100)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%V\n", ps)
}
*/
