package main

import (
	"net/http"
	"os"
)

func routes() {
	muxes := map[string]*http.ServeMux{"api": http.NewServeMux()}
	handlers := map[string]http.HandlerFunc{"ping": ping}
	muxes["api"].HandleFunc("/m", ping)
	mux := http.NewServeMux()
	mux.HandleFunc(os.Args[0], ping)
	mux.HandleFunc("/h", handlers["ping"])
}

func ping(w http.ResponseWriter, r *http.Request) {}
