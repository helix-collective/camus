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

// TODO the rest

var serverRoot = flag.String("serverRoot", "", "Path to the root directory in the prod machine")
var mode = flag.String("mode", "client", "'server' or 'client'")
var port = flag.Int("port", 8000, "port to serve on / connect to")
var runBackgroundCheck = flag.Bool("enforce", false, "Run background enforcer")

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
	server, err := NewServerImpl(
		*serverRoot,
		*runBackgroundCheck,
		*port)
	if err != nil {
		log.Fatal("NewServer:", err)
	}
	rpcServer := &RpcServer{server}
	rpc.Register(rpcServer)
	rpc.HandleHTTP()

	// Localhost only, in case it's not behind a firewall!
	portStr := fmt.Sprintf("localhost:%d", *port)
	l, err := net.Listen("tcp", portStr)
	if err != nil {
		log.Fatal("failed to listen:", err)
	}
	fmt.Printf("Listening on %s\n", portStr)
	http.Serve(l, nil)
}

func clientMain() {
	client, err := NewClientImpl(*port)
	if err != nil {
		log.Fatal("NewClient:", err)
	}
	err = NewTerminalClient(flag.CommandLine, client).Run()
	if err != nil {
		log.Fatal(err)
	}
}

func welcome() {
	println()
	println("  " + QUOTES[time.Now().UnixNano()%int64(len(QUOTES))])
	println("      -- Camus")
	println()
}

func setupChannel(remotePort int, login string) int {
	localPort := remotePort + 1 // todo: Just get some free port
	cmd := exec.Command("ssh", login, fmt.Sprintf("-L%d:localhost:%d", localPort, remotePort))
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
