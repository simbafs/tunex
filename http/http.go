package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

// delHopHeaders delete hop-by-hop headers to the backend.
func delHopHeaders(header http.Header) {
	hopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te", // canonicalized version of "TE"
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

type ProxyDest struct {
	Host    string
	IsHttps bool
}

type Proxy struct {
	proxyMap map[string]ProxyDest
}

func (p *Proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	log.Println("Got request", req.URL.String())

	if dest, ok := p.proxyMap[req.Host]; ok {
		p.doProxy(dest.Host, dest.IsHttps, wr, req)
		return
	}

	wr.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(wr, "<h1>No proxy for %s</h1>", req.Host)
}

func (p *Proxy) doProxy(to string, isHttps bool, wr http.ResponseWriter, req *http.Request) {
	// also see https://cs.opensource.google/go/go/+/refs/tags/go1.20.6:src/net/http/httputil/reverseproxy.go
	log.Printf("Proxying %s to %s, path: %s\n", req.Host, to, req.URL.Path)

	// Rewrite Request

	req.RequestURI = "" // Request.RequestURI can't be set in client requests
	if isHttps {
		req.URL.Scheme = "https"
	} else {
		req.URL.Scheme = "http"
	}
	req.Host = "" // this will be replace by req.URL.Host
	req.URL.Host = to

	delHopHeaders(req.Header)
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		appendHostToXForwardHeader(req.Header, clientIP)
	}

	// do request

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		fmt.Fprintf(wr, "Error: %s", err)
		return
	}

	// Rewrite Response

	delHopHeaders(res.Header)

	copyHeader(wr.Header(), res.Header)

	wr.WriteHeader(res.StatusCode)
	io.Copy(wr, res.Body)
}

func main() {
	addr := "127.0.0.1:3000"

	handler := &Proxy{
		proxyMap: map[string]ProxyDest{
			"localhost:3000": {
				Host:    "localhost:4000",
				IsHttps: false,
			},
			"github.localhost:3000": {
				Host:    "github.com",
				IsHttps: false,
			},
			"youtube.localhost:3000": {
				Host:    "youtube.com",
				IsHttps: false,
			},
		},
	}

	log.Println("Starting proxy server on", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
