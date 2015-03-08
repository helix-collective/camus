package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
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
var mode = flag.String("mode", "client", "'server' or 'client'")
var port = flag.String("port", fmt.Sprintf(":%d", CAMUS_PORT),
	"port to serve on / connect to")

func main() {
	welcome()

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
	println("Parsing deploy config")
	app := ApplicationFromConfig("deploy.json")
	//println("Setting up channel")
	//localPort := setupChannel("localhost")

	localPort := CAMUS_PORT

	serverAddr := fmt.Sprintf("localhost:%d", localPort)
	fmt.Printf("dialing %s\n", serverAddr)
	client, err := rpc.DialHTTP("tcp", serverAddr)
	if err != nil {
		log.Fatal("dialing:", err)
	}

	cmd := flag.Arg(0)

	build := func() {
		cmd := exec.Command("sh", "-c", app.BuildCmd())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}

	push := func(server string) string {
		req := &NewDeployDirRequest{}
		var reply NewDeployDirResponse

		err := client.Call("RpcServer.NewDeployDir", req, &reply)
		if err != nil {
			log.Fatal("RPC:", err)
		}

		target := fmt.Sprintf("%s:%s", app.SshTarget(server), reply.Path)

		println("RSYNC")
		cmd := exec.Command("rsync", "-az",
			fmt.Sprintf("%s/", app.BuildOutputDir()),
			target)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()

		println("RSYNC DONE")

		if err != nil {
			log.Fatalf("Failed to rsync: %s", err)
		}

		return reply.DeployId
	}

	deploy := func() string {
		println("===== BUILDING =====")
		build()

		println("===== PUSHING =====")
		return push("prod")

	}

	run := func(deployId string) {
		req := &RunRequest{deployId}
		var reply RunReply
		err := client.Call("RpcServer.Run", req, &reply)
		if err != nil {
			log.Fatal("RPC:", err)
		}

	}

	listDeploys := func() {
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

	if cmd == "deploy" {
		deploy()
	} else if cmd == "run" {
		run(deploy())
	} else if cmd == "list" {
		listDeploys()
	} else {
		log.Fatalf("Unrecognized command: '%s'", cmd)
	}

}

func welcome() {
	println("--------")
	println(QUOTES[int(time.Now().UnixNano())%len(QUOTES)])
	println("--------")
}

func setupChannel(login string) int {
	port := CAMUS_PORT
	localPort := port + 1
	cmd := exec.Command("ssh", login, fmt.Sprintf("-L%d:localhost:%d", localPort, port))
	_, err := cmd.StdinPipe()
	err = cmd.Start()

	if err != nil {
		log.Fatalf("%s", err)
	}

	return localPort
}

func sleepSeconds(seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
}
