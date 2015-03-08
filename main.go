package main

import "fmt"

type Deploy struct {
	Id   string
	Note string
	Port int // -1 for not running
}

type Label string

type Server interface {
	ListLabels() ([]Label, error)

	ListDeploys() ([]Deploy, error)

	Run(deployId string) error

	Stop(deployId string) error

	Label(deployId string, label Label) error

	// TODO Maintenance mode
}

/*

1. build
2. tag & push tag to git
3.

1. push binary (rsync)
   - build
   - tag & push tag to git
   - rsync binary
2. bring up binary (Run())
3. set to live (Label())


Application defines:
- build command (and tell us where the dir is)
- run command (with substitution for port)
- status check endpoint

*/

type Application struct {
	BuildCmd string

	BuildOutputDir string

	// needs a %PORT% part for port subsitution
	RunCmd string

	StatusEndpoint string

	// e.g. user@host  (no path)
	SshTarget string
}

func gitTag( /*args*/ ) {
}
func rsync( /*args*/ ) {
}

// TODO the rest

func main() {
	fmt.Println("To be")
}
