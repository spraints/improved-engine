package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "address to listen on")
	certDir := flag.String("certdir", "certs", "generate certs (if needed) and store them in this dir")
	var sans []string
	flag.Func("san", "add another alt name or IP to the generated cert", func(s string) error {
		sans = append(sans, s)
		return nil
	})
	flag.Parse()

	certFile, keyFile, err := getCerts(*certDir, sans)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %v", listener.Addr())

	mux := http.NewServeMux()
	server := &http.Server{
		Handler: reqLog(mux),
	}
	if err := server.ServeTLS(listener, certFile, keyFile); err != nil {
	}
}

func reqLog(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := &loggingWriter{w: w, r: r, t: time.Now()}
		defer ww.done()
		h.ServeHTTP(ww, r)
	})
}

type loggingWriter struct {
	w      http.ResponseWriter
	r      *http.Request
	t      time.Time
	status int32
	logged int32
}

func (w *loggingWriter) done() {
	if atomic.CompareAndSwapInt32(&w.logged, 0, 1) {
		log.Printf("%v %v -> %v (%v)", w.r.Method, w.r.RequestURI, atomic.LoadInt32(&w.status), time.Since(w.t))
	}
}

var _ http.ResponseWriter = &loggingWriter{}

// Header implements http.ResponseWriter.
func (w *loggingWriter) Header() http.Header {
	return w.w.Header()
}

// Write implements http.ResponseWriter.
func (w *loggingWriter) Write(data []byte) (int, error) {
	atomic.CompareAndSwapInt32(&w.status, 0, 200)
	return w.w.Write(data)
}

// WriteHeader implements http.ResponseWriter.
func (w *loggingWriter) WriteHeader(statusCode int) {
	atomic.StoreInt32(&w.status, int32(statusCode))
	w.w.WriteHeader(statusCode)
}

// getCerts parses or generates a server cert.
func getCerts(dir string, extraSans []string) (string, string, error) {
	certFile := filepath.Join(dir, "server.crt")
	certKey := filepath.Join(dir, "server.key")

	if certExists(certFile, certKey, extraSans, nil) {
		return certFile, certKey, nil
	}

	return "", "", fmt.Errorf("todo: generate certs")
}

func certExists(certFile, keyFile string, extraSans []string, ips []net.IP) bool {
	if _, err := os.Stat(keyFile); err != nil {
		return false
	}

	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		return false
	}

	block, rest := pem.Decode(certBytes)
	if len(rest) != 0 {
		// there should only be one cert.
		return false
	}

	if block.Type != "CERTIFICATE" {
		return false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}

        certHasSAN := func(subj string) bool {
          for _, s := range cert.DNSNames {
            if s == subj {
              return true
            }
          }
          return false
        }

	for _, subj := range extraSans {
          if !certHasSAN(subj) {
            return false
          }
	}

        certHasIP := func(ip net.IP) bool {
          for _, i := range cert.IPAddresses

	return true
}
