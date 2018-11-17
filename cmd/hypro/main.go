package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/chuangbo/hypro"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s -server hypro.cloud -domain myapp.hypro.cloud -target http://localhost:8080\n", os.Args[0])
}

func main() {
	domain := flag.String("domain", "", "Domain you would like to use, e.g. `myapp.hypro.cloud`")
	server := flag.String("server", "", "Server address, e.g. hypro.cloud")
	serverPort := flag.Int("server-port", 49776, "Server port")
	target := flag.String("target", "", "Forward target, e.g. http://localhost:8080")
	certFile := flag.String("cert", "", "Server certificate file to verify connection, e.g. hypro.crt (default: system root ca)")
	insecure := flag.Bool("insecure", false, "Allow connections to hypro server without certs")
	flag.Parse()

	if *target == "" || *domain == "" || *server == "" {
		usage()
		return
	}

	if err := hypro.DialAndServeReverseProxy(*server, *serverPort, *certFile, *domain, *target, *insecure); err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to the server: %v\n", err)
		return
	}
}
