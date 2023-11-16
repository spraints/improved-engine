package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
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

	var certFile, keyFile string
	if *certDir != "" {
		if f, k, err := generateCerts(*certDir, sans); err != nil {
			log.Fatal(err)
		} else {
			certFile = f
			keyFile = k
		}
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

func generateCerts(dir string, extraSans []string) (string, string, error) {
	certFile := filepath.Join(dir, "server.crt")
	certKey := filepath.Join(dir, "server.key")

	if certExists(certFile, certKey, extraSans) {
		return certFile, certKey, nil
	}

	return "", "", fmt.Errorf("todo: generate certs")
}

func certExists(cert, key string, extraSans []string) bool {
	// todo
	return false
}
