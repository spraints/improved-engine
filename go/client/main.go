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

	dialTimeout := flag.Duration("dial-timeout", 100*time.Millisecond, "dial timeout for http client")
	idleTimeout := flag.Duration("idle-timeout", 10*time.Second, "idle timeout for http client")
	readIdleTimeout := flag.Duration("read-idle-timeout", 2*time.Second, "read idle timeout for http2 client")
	writeByteTimeout := flag.Duration("write-byte-timeout", time.Second, "write byte timeout for http2 client")
	pingTimeout := flag.Duration("ping-timeout", 8*time.Second, "ping timeout for http2 client")
	threads := flag.Int("threads", 1, "number of concurrent clients to run")
	interval := flag.Duration("interval", 0, "time between requests")
	verbose := flag.Bool("verbose", false, "report every response")

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
	http2Transport.WriteByteTimeout = *writeByteTimeout
	http2Transport.PingTimeout = *pingTimeout

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	client := &http.Client{Transport: transport}

	log.Printf("starting %d goroutines...", *threads)

	var wg sync.WaitGroup
	wg.Add(*threads)
	for i := 0; i < *threads; i++ {
		go func(i int) {
			defer wg.Done()
			doClient(ctx, i, client, url, *interval, *verbose)
		}(i)
	}
	wg.Wait()
}

func doClient(ctx context.Context, i int, client *http.Client, url string, interval time.Duration, verbose bool) {
	start := time.Now()
	lastResp := time.Time{}
	var reqs uint64

	defer func() {
		duration := time.Since(start)
		var perReq time.Duration
		if reqs > 0 {
			perReq = duration / time.Duration(reqs)
		}
		if lastResp.IsZero() {
			log.Printf("[%d] %d reqs in %v (%v/req)", i, reqs, duration, perReq)
		} else {
			timeSinceLastResp := time.Since(lastResp)
			log.Printf("[%d] %d reqs in %v (%v/req) (last resp %v ago)", i, reqs, duration, perReq, timeSinceLastResp)
		}
	}()

	wait := func() bool { return true }
	if interval > 0 {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		wait = func() bool {
			select {
			case <-ctx.Done():
				return false
			case <-ticker.C:
				return true
			}
		}
	}

	for {
		reqs += 1
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("[%d] fatal: %v", i, err)
			}
			return
		}

		reqStart := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("[%d] fatal: %v", i, err)
			}
			return
		}

		reqDur := time.Since(reqStart)
		if verbose {
			log.Printf("[%d] GET %v -> %v (%v)",
				i, url, resp.StatusCode, reqDur)
		}

		lastResp = time.Now()
		resp.Body.Close()

		if !wait() {
			return
		}
	}
}
