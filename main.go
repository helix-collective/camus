package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os/exec"
	"path"
	"time"
)

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

const CAMUS_PORT = 9966

type RpcServer struct {
	server *ServerImpl
}

type ListDeploysRequest struct{}
type ListDeploysReply struct {
	Deploys []Deploy
}

func (s *RpcServer) ListDeploys(arg ListDeploysRequest, reply *ListDeploysReply) error {
	deploys, err := s.server.ListDeploys()
	if err != nil {
		return err
	}
	reply.Deploys = deploys
	return nil
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

func gitTag( /*args*/ ) {
}
func rsync( /*args*/ ) {
}

// TODO the rest

var serverRoot = flag.String("serverRoot", "", "Path to the root directory in the prod machine")
var mode = flag.String("mode", "server", "'server' or 'client'")
var port = flag.String("port", ":1234", "port to serve on / connect to")

func main() {
	welcome()

	app := ApplicationFromConfig("testapp/deploy.json")
	fmt.Printf("Build cmd: '%s'\n", app.BuildCmd())

	setupChannel("localhost")

	flag.Parse()
	fmt.Printf("running in '%s' mode\n", *mode)
	if *mode == "server" {
		serverMain()
	} else {
		clientMain()
	}
}

func serverMain() {
	s := &RpcServer{
		server: &ServerImpl{
			root: *serverRoot,
		},
	}
	rpc.Register(s)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", *port)
	if err != nil {
		log.Fatal("failed to listen:", err)
	}
	http.Serve(l, nil)
}

func clientMain() {
	serverAddr := "localhost" + *port
	fmt.Printf("dialing %s\n", serverAddr)
	client, err := rpc.DialHTTP("tcp", serverAddr)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	args := &ListDeploysRequest{}
	var reply ListDeploysReply
	err = client.Call("RpcServer.ListDeploys", args, &reply)

	if err != nil {
		log.Fatal("RPC:", err)
	}

	for _, deploy := range reply.Deploys {
		fmt.Printf("%v\n", deploy)
	}

}

func welcome() {
	println("--------")
	println(QUOTES[int(time.Now().UnixNano())%len(QUOTES)])
	println("--------")
}

func setupChannel(login string) func() {
	port := CAMUS_PORT
	cmd := exec.Command("ssh", login, fmt.Sprintf("-L%d:localhost:%d", port, port))
	pipe, err := cmd.StdinPipe()
	err = cmd.Start()

	if err != nil {
		log.Fatalf("%s", err)
	}

	return func() {
		pipe.Close()
	}
}

func sleepSeconds(seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
}
