package main

import (
	"log"
	"net/http"
	reverseproxy "tunex/reverseProxy"
)

func main() {
	proxy := &reverseproxy.Proxy{
		Addr: "localhost:3000",
		ProxyMap: reverseproxy.ProxyMap{
			"hello": {
				Host:    "localhost:4000",
				IsHttps: false,
			},
		},
	}

	if err := http.ListenAndServe(proxy.Addr, proxy); err != nil {
		log.Fatalf("Could not listen on port 3000 %v", err)
	}
}
