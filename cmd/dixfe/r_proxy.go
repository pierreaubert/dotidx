package main

import (
	"fmt"
	"net/http"
	"strings"
)

func (f *Frontend) handleProxy(w http.ResponseWriter, r *http.Request) {
	relay := r.PathValue("relay")
	chain := r.PathValue("chain")
	if _, ok := f.config.Parachains[relay][chain]; !ok {
		http.Error(w, "Invalid relay or chain", http.StatusBadRequest)
		return
	}
	proxy := f.proxys[relay][chain]
	path := r.URL.Path
	path = strings.TrimPrefix(path, fmt.Sprintf("/proxy/%s/%s", relay, chain))
	r.URL.Path = path
	proxy.ServeHTTP(w, r)
}
