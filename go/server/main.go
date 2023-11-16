package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"
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
	flag.Parse()

	certFile, keyFile, err := getCerts(*certDir)
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
func getCerts(dir string) (string, string, error) {
	certFile := filepath.Join(dir, "server.crt")
	keyFile := filepath.Join(dir, "server.key")

	os.Mkdir(dir, 0755)
	os.Remove(certFile)
	os.Remove(keyFile)

	key, err := generateKey(keyFile)
	if err != nil {
		return "", "", err
	}

	_, err = generateCert(certFile, key)
	if err != nil {
		return "", "", err
	}

	return certFile, keyFile, nil
}

func generateKey(keyFile string) (*ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize private key for new certificate: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if keyPEM == nil || len(keyPEM) < 1 {
		return nil, fmt.Errorf("failed to PEM-encode generated certificate's key")
	}

	if err := os.WriteFile(keyFile, keyPEM, 0444); err != nil {
		return nil, err
	}

	return key, nil
}

func generateCert(certFile string, key *ecdsa.PrivateKey) (*x509.Certificate, error) {
	serialNumber, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, fmt.Errorf("failed to generate random serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:       []string{"Spraints"},
			OrganizationalUnit: []string{"Exp"},
			CommonName:         "localhost",
		},
		NotBefore:             time.Now().Add(-10 * time.Minute),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: false,
		IPAddresses: []net.IP{
			net.IPv4(127, 0, 0, 1),
		},
	}

	certDer, err := x509.CreateCertificate(rand.Reader, &template, &template, key.Public(), key)
	if err != nil {
		return nil, fmt.Errorf("failed to perform certificate generation")
	}

	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDer})
	if certPem == nil || len(certPem) < 1 {
		return nil, fmt.Errorf("failed to PEM-encode generated certificate")
	}

	if err := os.WriteFile(certFile, certPem, 0444); err != nil {
		return nil, err
	}

	return x509.ParseCertificate(certDer)
}
