package main

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
)

type TerminalClient struct {
	flags  *flag.FlagSet
	client Client
}

type Command func() error

func NewTerminalClient(flags *flag.FlagSet, client Client) *TerminalClient {
	return &TerminalClient{flags, client}
}

func (c *TerminalClient) Run() error {
	commands := map[string]Command{
		"deploy": c.deployCmd,
		"run":    c.runCmd,
		"list":   c.listCmd,
		"set":    c.setCmd,
	}
	cmdName := c.flags.Arg(0)
	cmd, ok := commands[cmdName]
	if !ok {
		return fmt.Errorf("Unknown command '%s'", cmdName)
	}
	err := cmd()
	if err != nil {
		return fmt.Errorf("Command '%s' failed: %s", cmdName, err)
	}
	return nil
}

func (c *TerminalClient) deployCmd() error {
	deployId, err := c.client.Push("prod")
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
	_, err := c.client.Run(deployId)
	if err != nil {
		return err
	}
	println("Ran")
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
