// Usage: go run ./frame-size client|server
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/http2"
)

const (
	addr = "127.0.0.1:5623"

	// Assume we're running from the project root and certs have been generated
	// with the server in 'script/server'.
	certFile = "certs/server.crt"
	keyFile  = "certs/server.key"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: go run ./frame-size client|server")
	}

	switch os.Args[1] {
	case "client":
		if err := client(); err != nil {
			log.Fatal(err)
		}
	case "server":
		if err := server(); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("Usage: go run ./frame-size client|server")
	}
}

func client() error {
	req, err := http.NewRequest("GET", "https://"+addr+"/", nil)
	if err != nil {
		return err
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	_, err = http2.ConfigureTransports(transport)
	if err != nil {
		return err
	}

	client := &http.Client{
		Transport: transport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("%s %s\n", resp.Proto, resp.Status)
	return nil
}

func server() error {
	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%v %v %v", r.Proto, r.Method, r.URL)
			fmt.Fprintf(w, "OK\n")
		}),
	}

	log.Println("Listening on", addr)

	return srv.ListenAndServeTLS(certFile, keyFile)
}
