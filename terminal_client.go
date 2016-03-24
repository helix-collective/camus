package main

import (
	"errors"
	"flag"
	"fmt"
	"sort"
	"strconv"
)

type TerminalClient struct {
	flags    *flag.FlagSet
	client   Client
	commands map[string]Command
}

type Command func() error

func NewTerminalClient(flags *flag.FlagSet, client Client) *TerminalClient {
	c := &TerminalClient{flags, client, make(map[string]Command)}
	c.commands["deploy"] = c.deployCmd
	c.commands["run"] = c.runCmd
	c.commands["list"] = c.listCmd
	c.commands["set"] = c.setCmd
	c.commands["help"] = c.helpCmd
	c.commands["stop"] = c.stopCmd
	// TODO(koz): Consider not exposing these in the terminal client.
	c.commands["cleanup"] = c.cleanupCmd
	c.commands["shutdown"] = c.shutdownCmd
	return c
}

func (c *TerminalClient) Run() error {
	cmdName := c.flags.Arg(0)
	if cmdName == "" {
		cmdName = "help"
	}
	cmd, ok := c.commands[cmdName]
	if !ok {
		fmt.Printf("Unknown command '%s'\n", cmdName)
		c.helpCmd()
		return nil
	}
	return cmd()
}

func (c *TerminalClient) helpCmd() error {
	fmt.Printf("usage: camus [-server] command args...\n\n")
	fmt.Printf("Available commands\n")
	cmdNames := []string{}
	for name, _ := range c.commands {
		cmdNames = append(cmdNames, name)
	}
	sort.StringSlice(cmdNames).Sort()
	for _, name := range cmdNames {
		fmt.Printf("  %s\n", name)
	}
	return nil
}

func (c *TerminalClient) deployCmd() error {
	if _, err := c.client.Build(); err != nil {
		return err
	}

	deployId := NewDeployId()
	if err := c.client.Push(deployId); err != nil {
		return err
	}

	fmt.Printf("Deployed '%s'\n", deployId)
	return nil
}

func (c *TerminalClient) runCmd() error {
	deployId := c.flags.Arg(1)
	if deployId == "" {
		return errors.New("Missing deploy id")
	}
	port, err := c.client.Run(deployId)
	if err != nil {
		return err
	}
	println(port)
	return nil
}

func (c *TerminalClient) setCmd() error {
	deployIdOrPort := c.flags.Arg(1)
	if deployIdOrPort == "" {
		return errors.New("Missing deploy id or port")
	}

	port, err := strconv.Atoi(deployIdOrPort)
	isPort := err == nil

	if isPort && port <= 0 {
		return errors.New("Invalid port")
	}

	if isPort {
		err = c.client.SetMainByPort(port)
	} else {
		err = c.client.SetMainById(deployIdOrPort)
	}

	if err != nil {
		return err
	}

	println("Active deploy set")
	return nil
}

func (c *TerminalClient) listCmd() error {
	deploys, err := c.client.ListDeploys()
	if err != nil {
		return err
	}
	fmt.Printf("Deploys:\n")

	tbl := TableDef{
		Columns: []ColumnDef{
			ColumnDef{"id", 25},
			ColumnDef{"pid", 5},
			ColumnDef{"tracked", 7},
			ColumnDef{"port", 4},
			ColumnDef{"st", 3},
			ColumnDef{"messages", 50},
		},
	}
	tbl.PrintHeader()

	for _, deploy := range deploys {
		tbl.PrintRow(
			deploy.Id,
			deploy.Pid,
			yn(deploy.Tracked),
			deploy.Port,
			deploy.Health,
			fmt.Sprintf("%v", deploy.Errors),
		)
	}
	return nil
}

func (c *TerminalClient) stopCmd() error {
	deployId := c.flags.Arg(1)
	if deployId == "" {
		return errors.New("Missing deploy id")
	}
	err := c.client.Stop(deployId)
	if err != nil {
		return err
	}
	println("killed")
	return nil
}

func (c *TerminalClient) cleanupCmd() error {
	c.client.KillUnknownProcesses()
	return nil
}

func (c *TerminalClient) shutdownCmd() error {
	c.client.Shutdown()
	return nil
}

func yn(b bool) string {
	if b {
		return "y"
	} else {
		return "n"
	}
}
