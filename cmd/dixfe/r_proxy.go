package main

import (
	"net/http"
	"net/url"
	"strings"
)

func (f *Frontend) handleProxy(w http.ResponseWriter, r *http.Request) {
	remote, _ := url.Parse(f.config.ChainReaderURL)
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/proxy")
	r.Host = remote.Host
	r.URL.Path = path
	f.proxy.ServeHTTP(w, r)
}
