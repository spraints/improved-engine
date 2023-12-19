// Usage: go run ./frame-size client|server
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

const (
	addr = "127.0.0.1:5623"

	// Assume we're running from the project root and certs have been generated
	// with the server in 'script/server'.
	certFile = "certs/server.crt"
	keyFile  = "certs/server.key"

	// How many requests to make from the client.
	numRequests = 15

	// How many clients to run in parallel.
	numClients = 2
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

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
	var wg sync.WaitGroup
	wg.Add(numClients)
	for i := 0; i < numClients; i++ {
		go func(i int) {
			defer wg.Done()
			if err := doClient(i); err != nil {
				log.Printf("client %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	return nil
}

func doClient(clientNum int) error {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	_, err := http2.ConfigureTransports(transport)
	if err != nil {
		return err
	}

	client := &http.Client{
		Transport: transport,
	}

	var wg sync.WaitGroup
	wg.Add(numRequests)
	for i := 0; i < numRequests; i++ {
		go func(i int) {
			defer wg.Done()
			doReq(client, clientNum, i)
		}(i)
	}
	wg.Wait()
	return nil
}

func doReq(client *http.Client, clientNum, reqNum int) error {
	url := fmt.Sprintf("https://%s/%d/%d", addr, clientNum, reqNum)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	log.Printf("%s (%v) -> %s %s\n", url, elapsed, resp.Proto, resp.Status)
	return nil
}

func server() error {
	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[%v] %v %v %v",
				r.RemoteAddr, r.Proto, r.Method, r.RequestURI)
			fmt.Fprintf(w, "OK\n")
		}),
	}

	log.Println("Listening on", addr)

	return srv.ListenAndServeTLS(certFile, keyFile)
}
