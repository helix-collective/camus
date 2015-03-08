package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os/exec"
	"time"
)

type Deploy struct {
	Id   string
	Note string
	Port int // -1 for not running
}

type Label string

const CAMUS_PORT = 9966

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
	s := &RpcServer{NewServerImpl(*serverRoot)}
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
