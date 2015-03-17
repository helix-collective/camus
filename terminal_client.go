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
	fmt.Printf("usage: camus -mode [server|client] command args...\n\n")
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
	_, err := c.client.Build()
	if err != nil {
		return err
	}

	deployId, err := c.client.Push()
	if err != nil {
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
	portStr := c.flags.Arg(1)
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return errors.New("Invalid port")
	}

	err = c.client.SetMainByPort(port)
	if err != nil {
		return err
	}
	println("Port set")
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

func yn(b bool) string {
	if b {
		return "y"
	} else {
		return "n"
	}
}
