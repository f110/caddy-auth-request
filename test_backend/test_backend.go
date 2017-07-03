package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		log.Print(*req)
	})
	log.Fatal(http.ListenAndServe("127.0.0.1:2015", nil))
}
