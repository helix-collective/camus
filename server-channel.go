package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type TargetBox interface {
	// Copy transfers all files from the local src directory, to the (potentially)
	// remove dst directory. It's used by the deploy command
	Copy(src string, dst string) error

	// Exec runs a given command on the remote machine
	Exec(command ...string) error

	// Close cleanups the connection to the target box we are operating on (eg.
	// disconnecting ssh tunnel)
	Close()
}

type CommandRunner func(command string, args ...string) error

// Setup port forwarding, and
type SshServerChannel struct {
	commandRunner CommandRunner
	cmd           *exec.Cmd
	sshPort       int
	ssh           string
}

// Setup port forwarding, and
type LocalServerChannel struct {
	commandRunner CommandRunner
}

func NewSshChannel(
	remotePort int,
	localPort int,
	sshPort int,
	login string,
	runner CommandRunner) (*SshServerChannel, error) {

	// create SSH tunnel
	cmd := exec.Command(
		"ssh", "-o", "StrictHostKeyChecking=no", "-p", strconv.Itoa(sshPort), login,
		fmt.Sprintf("-L%d:localhost:%d", localPort, remotePort))
	if _, err := cmd.StdinPipe(); err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	fmt.Printf("Opening connection to %s:%d -> camus@%d ..",
		login, sshPort, remotePort)

	for portFree(localPort) {
		print(".")
		sleepSeconds(1)
	}
	println()

	return &SshServerChannel{
		cmd:           cmd,
		commandRunner: runner,
		sshPort:       sshPort,
		ssh:           login,
	}, nil
}

func (s *SshServerChannel) Copy(src string, dst string) error {
	rsyncArgs := append(baseRsyncArgs(src), []string{
		"-e",
		fmt.Sprintf("ssh -p %d -o StrictHostKeyChecking=no", s.sshPort),
		s.ssh + ":" + dst,
	}...)

	if err := s.commandRunner("rsync", rsyncArgs...); err != nil {
		return err
	}

	return nil
}

func (s *SshServerChannel) Exec(command ...string) error {
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-p", strconv.Itoa(s.sshPort),
		s.ssh,
	}
	args := append(sshArgs, command...)
	return s.commandRunner("ssh", args...)
}

func (s *SshServerChannel) Close() {
	if err := s.cmd.Process.Kill(); err != nil {
		panic("Error killing ssh tunnel")
	}
}

//////

func NewLocalChannel(runner CommandRunner) *LocalServerChannel {
	return &LocalServerChannel{commandRunner: runner}
}

func (s *LocalServerChannel) Copy(src string, dst string) error {
	rsyncArgs := append(baseRsyncArgs(src), dst)

	if err := s.commandRunner("rsync", rsyncArgs...); err != nil {
		return err
	}

	return nil
}

func (s *LocalServerChannel) Exec(command ...string) error {
	return s.commandRunner("bash", "-c", strings.Join(command, " "))
}

func (s *LocalServerChannel) Close() {
	// nothing to close
}

//////

func baseRsyncArgs(src string) []string {
	return []string{
		"-azv", "--delete",
		src + "/",
	}
}
