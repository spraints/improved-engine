package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/http2"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [OPTIONS] HOST:PORT\n", os.Args[0])
		flag.PrintDefaults()
	}

	// certDir := flag.String("certdir", "certs", "dir where the server generated its self-signed cert")
	dialTimeout := flag.Duration("dial-timeout", 100*time.Millisecond, "dial timeout for http client")
	idleTimeout := flag.Duration("idle-timeout", 10*time.Second, "idle timeout for http client")
	readIdleTimeout := flag.Duration("read-idle-timeout", 10*time.Second, "read idle timeout for http2 client")
	pingTimeout := flag.Duration("ping-timeout", 10*time.Second, "ping timeout for http2 client")

	flag.Parse()

	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(1)
	}
	addr := flag.Args()[0]

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	dialer := &net.Dialer{
		Timeout: *dialTimeout,
	}

	transport := &http.Transport{
		DialContext:     dialer.DialContext,
		TLSClientConfig: tlsConfig,
		IdleConnTimeout: *idleTimeout,
	}

	http2Transport, err := http2.ConfigureTransports(transport)
	if err != nil {
		log.Fatal(err)
	}
	http2Transport.ReadIdleTimeout = *readIdleTimeout
	http2Transport.PingTimeout = *pingTimeout

	client := &http.Client{Transport: transport}
	log.Printf("todo: make a bunch of requests %v %v", addr, client)
}
