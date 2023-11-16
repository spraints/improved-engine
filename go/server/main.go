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
	"strconv"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "address to listen on")
	certDir := flag.String("certdir", "certs", "generate certs (if needed) and store them in this dir")
	maxStreams := flag.Int("max-streams", 0, "max concurrent streams for http/2 server (0 uses Go's default)")
	verbose := flag.Bool("verbose", false, "log every request")
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

	mux.HandleFunc("/slow", func(_ http.ResponseWriter, r *http.Request) {
		if sec, err := strconv.ParseUint(r.FormValue("s"), 10, 8); err == nil {
			time.Sleep(time.Duration(sec) * time.Second)
		} else {
			time.Sleep(time.Second)
		}
	})

	mux.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		var size, blockSize uint64
		size, _ = strconv.ParseUint(r.FormValue("bytes"), 10, 32)
		blockSize, _ = strconv.ParseUint(r.FormValue("bs"), 10, 32)
		if blockSize < 1 {
			blockSize = 1024 * 1024
		}
		data := make([]byte, 0, int(blockSize))
		for i := 0; i < int(blockSize); i++ {
			data = append(data, 'a')
		}
		rem := int(size)
		for rem > 0 {
			toSend := data
			if rem < len(toSend) {
				toSend = toSend[:rem]
			}
			if sent, err := w.Write(toSend); err != nil {
				log.Println(err)
				return
			} else {
				rem -= sent
			}
		}
	})

	var h http.Handler = mux
	if *verbose {
		h = reqLog(h)
	}

	server := &http.Server{Handler: h}

	http2.ConfigureServer(server, &http2.Server{
		MaxConcurrentStreams: uint32(*maxStreams),
	})

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
	length uint64
}

func (w *loggingWriter) done() {
	if atomic.CompareAndSwapInt32(&w.logged, 0, 1) {
		log.Printf("[%v %v] %v %v -> %v (%v, %d bytes)",
			w.r.Proto, w.r.RemoteAddr,
			w.r.Method, w.r.RequestURI,
			atomic.LoadInt32(&w.status), time.Since(w.t), atomic.LoadUint64(&w.length))
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
	atomic.AddUint64(&w.length, uint64(len(data)))
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
