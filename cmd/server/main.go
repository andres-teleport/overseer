package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/andres-teleport/overseer/api/server"
)

func main() {
	log.SetFlags(0)

	var listen, key, cert, ca string
	flag.StringVar(&listen, "listen", "localhost:9999", "listening address and port")
	flag.StringVar(&key, "key", "certs/server.key", "path to the private key")
	flag.StringVar(&cert, "cert", "certs/server.crt", "path to the certificate")
	flag.StringVar(&ca, "ca", "certs/ca.crt", "path to the certificate of the Certificate Authority")
	flag.Parse()

	fmt.Printf("Listening on %s.\n", listen)

	srv, err := server.NewServer(listen, key, cert, ca)
	if err != nil {
		log.Fatal(err)
	}

	if err = srv.Serve(); err != nil {
		log.Fatal(err)
	}
}
