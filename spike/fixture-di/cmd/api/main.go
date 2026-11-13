package main

import "net/http"

func main() {
	h := initApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tx", h.Create)
	_ = http.ListenAndServe(":8080", mux)
}
