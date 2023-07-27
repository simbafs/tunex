package reverseproxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

type DoProxyConfig struct {
	client  *http.Client
	to      string
	isHttps bool
	wr      http.ResponseWriter
	req     *http.Request
}

func DoProxy(config DoProxyConfig) {
	// also see https://cs.opensource.google/go/go/+/refs/tags/go1.20.6:src/net/http/httputil/reverseproxy.go

	// Rewrite Request

	config.req.RequestURI = "" // Request.RequestURI can't be set in client requests
	if config.isHttps {
		config.req.URL.Scheme = "https"
	} else {
		config.req.URL.Scheme = "http"
	}
	config.req.Host = "" // this will be replace by req.URL.Host
	config.req.URL.Host = config.to

	delHopHeaders(config.req.Header)
	if clientIP, _, err := net.SplitHostPort(config.req.RemoteAddr); err == nil {
		appendHostToXForwardHeader(config.req.Header, clientIP)
	}

	// do request

	res, err := config.client.Do(config.req)
	if err != nil {
		log.Println("Error:", err)
		fmt.Fprintf(config.wr, "<h1>Something error</h1>")
		return
	}

	// Rewrite Response

	delHopHeaders(res.Header)

	copyHeader(config.wr.Header(), res.Header)

	config.wr.WriteHeader(res.StatusCode)
	io.Copy(config.wr, res.Body)
	res.Body.Close()
}

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
