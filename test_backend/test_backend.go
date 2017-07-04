package main

import (
	"log"
	"net/http"
	"net/http/httputil"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		buf, _ := httputil.DumpRequest(req, false)
		log.Print(string(buf))
		w.WriteHeader(http.StatusForbidden)
	})
	log.Fatal(http.ListenAndServe("127.0.0.1:2015", nil))
}
