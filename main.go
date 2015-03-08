package main

import "flag"
import "fmt"
import "io/ioutil"
import "log"
import "path"

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

const deployPath = "deploys"

type ServerImpl struct {
	root string
}

func (s *ServerImpl) ListDeploys() ([]Deploy, error) {
	infos, err := ioutil.ReadDir(path.Join(s.root, deployPath))
	if err != nil {
		return nil, err
	}
	var result []Deploy
	for _, info := range infos {
		result = append(result, Deploy{
			Id:   info.Name(),
			Port: -1,
		})
	}
	return result, nil
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
	SshTarget map[string]string
}

func gitTag( /*args*/ ) {
}
func rsync( /*args*/ ) {
}

// TODO the rest

var serverRoot = flag.String("serverRoot", "", "Path to the root directory in the prod machine")

func main() {
	flag.Parse()
	server := ServerImpl{
		root: *serverRoot,
	}
	deploys, err := server.ListDeploys()
	if err != nil {
		log.Fatalf("Failed to list deploys: %s", err)
	}

	for _, deploy := range deploys {
		fmt.Printf("%v\n", deploy)
	}
}
