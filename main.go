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

const CAMUS_PORT = 9966

// TODO the rest

var serverRoot = flag.String("serverRoot", "", "Path to the root directory in the prod machine")
var mode = flag.String("mode", "client", "'server' or 'client'")
var port = flag.String("port", fmt.Sprintf(":%d", CAMUS_PORT),
	"port to serve on / connect to")

func main() {
	welcome()
	flag.Parse()
	if *mode == "server" {
		serverMain()
	} else {
		clientMain()
	}
}

func serverMain() {
	server, err := NewServerImpl(*serverRoot)
	if err != nil {
		log.Fatal("NewServer:", err)
	}
	rpcServer := &RpcServer{server}
	rpc.Register(rpcServer)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", *port)
	if err != nil {
		log.Fatal("failed to listen:", err)
	}
	fmt.Printf("Listening on %s\n", *port)
	http.Serve(l, nil)
}

func clientMain() {
	client, err := NewClientImpl()
	if err != nil {
		log.Fatal("NewClient:", err)
	}
	err = NewTerminalClient(flag.CommandLine, client).Run()
	if err != nil {
		log.Fatal(err)
	}
}

func welcome() {
	println(QUOTES[time.Now().UnixNano()%int64(len(QUOTES))])
	println("  -- Camus")
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
