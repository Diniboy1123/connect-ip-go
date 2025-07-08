// HTTP/2 and HTTP/3 Examples for connect-ip-go

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	connectip "github.com/Diniboy1123/connect-ip-go"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/yosida95/uritemplate/v3"
	"golang.org/x/net/http2"
)

func exampleHTTP3Client() {
	// HTTP/3 client example (existing functionality)
	tr := &http3.Transport{
		EnableDatagrams: true,
	}
	defer tr.Close()

	// Establish connection
	conn, err := quic.DialAddr(
		context.Background(),
		"example.com:443",
		&tls.Config{
			ServerName: "example.com",
		},
		&quic.Config{EnableDatagrams: true},
	)
	if err != nil {
		log.Fatal(err)
	}

	clientConn := tr.NewClientConn(conn)
	template := uritemplate.MustNew("https://example.com/.well-known/masque/ip/")
	
	// Dial using HTTP/3
	proxyConn, resp, err := connectip.Dial(
		context.Background(),
		clientConn,
		template,
		"HTTP/3.0",
		http.Header{},
		false,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer proxyConn.Close()
	defer resp.Body.Close()

	fmt.Println("HTTP/3 connection established")
}

func exampleHTTP2Client() {
	// HTTP/2 client example (new functionality)
	
	// Create HTTP/2 transport
	transport := &http2.Transport{
		TLSClientConfig: &tls.Config{
			ServerName: "example.com",
		},
	}

	// Create HTTP client with HTTP/2 transport
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	template := uritemplate.MustNew("https://example.com/.well-known/masque/ip/")
	
	// Dial using HTTP/2
	proxyConn, resp, err := connectip.DialHTTP2(
		context.Background(),
		client,
		template,
		"HTTP/2.0",
		http.Header{},
		false,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer proxyConn.Close()
	defer resp.Body.Close()

	fmt.Println("HTTP/2 connection established")
}

func exampleAutoClient() {
	// Auto-detecting client example
	
	// This will automatically choose HTTP/2 for non-HTTP/3 transports
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			ServerName: "example.com",
		},
	}

	template := uritemplate.MustNew("https://example.com/.well-known/masque/ip/")
	
	// DialAuto will detect the transport type and choose the appropriate method
	proxyConn, resp, err := connectip.DialAuto(
		context.Background(),
		transport,
		template,
		"HTTP/2.0",
		http.Header{},
		false,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer proxyConn.Close()
	defer resp.Body.Close()

	fmt.Println("Auto-detected connection established")
}

func exampleHTTP2Server() {
	// HTTP/2 server example
	template := uritemplate.MustNew("https://localhost:8443/.well-known/masque/ip/")
	proxy := &connectip.Proxy{}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/masque/ip/", func(w http.ResponseWriter, r *http.Request) {
		// Parse the CONNECT-IP request
		req, err := connectip.ParseRequest(r, template, "HTTP/2.0")
		if err != nil {
			if parseErr, ok := err.(*connectip.RequestParseError); ok {
				http.Error(w, parseErr.Error(), parseErr.HTTPStatus)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Establish the proxy connection
		conn, err := proxy.Proxy(w, r, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		// Handle the connection...
		fmt.Println("HTTP/2 proxy connection established")
	})

	// Configure HTTP/2 server
	server := &http.Server{
		Addr:    ":8443",
		Handler: mux,
		TLSConfig: &tls.Config{
			// Add your TLS configuration here
		},
	}

	// Configure HTTP/2
	http2.ConfigureServer(server, &http2.Server{})

	fmt.Println("Starting HTTP/2 server on :8443")
	log.Fatal(server.ListenAndServeTLS("cert.pem", "key.pem"))
}

func main() {
	fmt.Println("connect-ip-go now supports both HTTP/2 and HTTP/3!")
	
	// Uncomment the example you want to run:
	// exampleHTTP2Client()
	// exampleHTTP3Client()
	// exampleAutoClient()
	// exampleHTTP2Server()
}