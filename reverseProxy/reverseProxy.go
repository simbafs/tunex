package reverseproxy

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

type ProxyMap map[string]ProxyDest

// Proxy is a HTTP reverse proxy.
// http.ListenAndServe(":3000", &Proxy{...})
type Proxy struct {
	// Addr should be in the form of "host:port"
	Addr     string
	ProxyMap ProxyMap
}

func (p *Proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	subdomain := strings.Split(req.Host, ".")[0]

	if dest, ok := p.ProxyMap[subdomain]; ok {
		log.Printf("subdomain: %s -> %s", subdomain, dest.Host)
		p.doProxy(dest.Host, dest.IsHttps, wr, req)
		return
	}

	log.Printf("subdomain: %s -> unknown", subdomain)
	wr.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(wr, "<h1>No proxy for %s</h1>", req.Host)
}

func (p *Proxy) doProxy(to string, isHttps bool, wr http.ResponseWriter, req *http.Request) {
	// also see https://cs.opensource.google/go/go/+/refs/tags/go1.20.6:src/net/http/httputil/reverseproxy.go

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
		fmt.Fprintf(wr, "<h1>Something error</h1>")
		return
	}

	// Rewrite Response

	delHopHeaders(res.Header)

	copyHeader(wr.Header(), res.Header)

	wr.WriteHeader(res.StatusCode)
	io.Copy(wr, res.Body)
}
