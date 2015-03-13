package main

import (
	"errors"
	"flag"
	"fmt"
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

func (c *TerminalClient) listCmd() error {
	deploys, err := c.client.ListDeploys()
	if err != nil {
		return err
	}
	fmt.Printf("Deploys:\n")
	for _, deploy := range deploys {
		fmt.Printf("%-25s ", deploy.Id)
		fmt.Printf(yn(deploy.Tracked))
		fmt.Printf(" %4d ", deploy.Port)
		fmt.Printf(" %4d ", deploy.Health)
		fmt.Printf(" %25s ", fmt.Sprintf("%v", deploy.Errors))
		println()
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
