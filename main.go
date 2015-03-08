package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os/exec"
	"time"
)

const CAMUS_PORT = 9966

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
	client := NewClientImpl()

	cmd := flag.Arg(0)

	var err error

	if cmd == "deploy" {
		deployId, err := client.Push("prod")
		if err == nil {
			println("Deploy id ", deployId)
		}
	} else if cmd == "run" {
		deployId := flag.Arg(1)
		if deployId == "" {
			err = errors.New("Missing deploy id")
		} else {
			_, err = client.Run(flag.Arg(1))
			if err == nil {
				println("Ran")
			}
		}

	} else if cmd == "list" {
		deploys, err := client.ListDeploys()
		if err == nil {
			for _, deploy := range deploys {
				fmt.Printf("%v\n", deploy)
			}
		}
	} else {
		log.Fatalf("Unrecognized command: '%s'", cmd)
	}

	if err != nil {
		log.Fatal("Error: ", err)
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
