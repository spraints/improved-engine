package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [OPTIONS] URL\n", os.Args[0])
		flag.PrintDefaults()
	}

	// certDir := flag.String("certdir", "certs", "dir where the server generated its self-signed cert")
	dialTimeout := flag.Duration("dial-timeout", 100*time.Millisecond, "dial timeout for http client")
	idleTimeout := flag.Duration("idle-timeout", 10*time.Second, "idle timeout for http client")
	readIdleTimeout := flag.Duration("read-idle-timeout", 10*time.Second, "read idle timeout for http2 client")
	pingTimeout := flag.Duration("ping-timeout", 10*time.Second, "ping timeout for http2 client")
	threads := flag.Int("threads", 1, "number of concurrent clients to run")

	flag.Parse()

	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(1)
	}
	url := flag.Args()[0]

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

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	client := &http.Client{Transport: transport}
	var wg sync.WaitGroup
	wg.Add(*threads)
	for i := 0; i < *threads; i++ {
		go func(i int) {
			defer wg.Done()
			doClient(ctx, i, client, url)
		}(i)
	}
	wg.Wait()
}

func doClient(ctx context.Context, i int, client *http.Client, url string) {
	start := time.Now()
	var reqs uint64

	defer func() {
		log.Printf("[%d] %d reqs in %v", i, reqs, time.Since(start))
	}()

	for {
		reqs += 1
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("[%d] fatal: %v", i, err)
			}
			return
		}
		_, err = client.Do(req)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("[%d] fatal: %v", i, err)
			}
			return
		}
	}
}
