package main

import (
	"net/http"

	"github.com/h-dav/cache-proxy/internal/proxy"
)

func main() {
	proxy := proxy.New("localhost:8080")
	proxy.Origin = "localhost:9090"

	proxy.FlushCache()

	http.Handle("/", proxy)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
