package main

import (
	"log"
	"net/http"
	"os"

	"github.com/elazarl/goproxy"
)

const (
	defaultPort = "8080"
)

func main() {
	port := os.Getenv("KUBECTL_PORTAL_PROXY_PORT")
	if port == "" {
		port = defaultPort
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true

	log.Printf("Started kubectl-portal-proxy")
	log.Fatal(http.ListenAndServe(":"+port, proxy))
}
