package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/chuangbo/hypro"
)

func main() {
	grpcAddr := flag.String("listen", ":49776", "API server listen address")
	httpAddr := flag.String("http", ":80", "HTTP server listen address")
	certFile := flag.String("cert", "", "Server certificate file")
	keyFile := flag.String("key", "", "Server certificate key file")
	flag.Parse()

	err := hypro.ListenAndServe(*grpcAddr, *httpAddr, *certFile, *keyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not start the server at %s %s: %v\n", *grpcAddr, *httpAddr, err)
	}
}
