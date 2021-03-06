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
	if err := c.client.Build(); err != nil {
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
	if err := c.client.Run(deployId); err != nil {
		return err
	}
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
		err = c.client.SetActiveByPort(port)
	} else {
		err = c.client.SetActiveById(deployIdOrPort)
	}

	if err != nil {
		return err
	}

	println("Active deploy set")
	return nil
}

type ByDeployId []*Deploy

func (ds ByDeployId) Len() int           { return len(ds) }
func (ds ByDeployId) Swap(i, j int)      { ds[i], ds[j] = ds[j], ds[i] }
func (ds ByDeployId) Less(i, j int) bool { return ds[i].Id < ds[j].Id }

func (c *TerminalClient) listCmd() error {
	deploys, err := c.client.ListDeploys()
	if err != nil {
		return err
	}

	sort.Sort(ByDeployId(deploys))

	fmt.Printf("Deploys:\n")

	tbl := TableDef{
		Columns: []ColumnDef{
			ColumnDef{"   id", 45},
			ColumnDef{"pid", 5},
			ColumnDef{"tracked", 7},
			ColumnDef{"port", 4},
			ColumnDef{"st", 3},
			ColumnDef{"messages", 50},
		},
	}
	tbl.PrintHeader()

	prevId := ""
	for _, d := range deploys {
		id := d.Id

		// Only show Id once per group of related deploys
		if d.Id == prevId {
			id = ""
		}

		tbl.PrintRow(
			fmt.Sprintf("%s%s", activePointer(d.Set), id),
			d.Pid,
			yn(d.Tracked),
			d.Port,
			d.Health,
			fmt.Sprintf("%v", d.Errors),
		)

		prevId = d.Id
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

func activePointer(b bool) string {
	if b {
		return " * "
	} else {
		return "   "
	}
}
