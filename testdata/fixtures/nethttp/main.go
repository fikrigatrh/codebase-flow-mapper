package main

import (
	"net/http"

	"example.com/nethttp/internal/api"
)

func main() {
	mux := http.NewServeMux()
	u := &api.Users{}
	mux.HandleFunc("GET /users/{id}", u.Get)
	mux.HandleFunc("POST  /users", api.CreateUser)
	mux.Handle("/static/", http.HandlerFunc(api.Static))
	mux.Handle("GET /health", api.Health{})
	mux.Handle("DELETE /users/{id}", api.Logging(api.RequireRole("admin", http.HandlerFunc(u.Delete))))
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {})
	mux.Handle("GET /files/", http.StripPrefix("/files/", http.FileServer(http.Dir("."))))
	var h http.Handler = api.Health{}
	mux.Handle("/iface", h)
	api.Mount(mux)
	http.HandleFunc("GET /default", api.CreateUser)
	http.DefaultServeMux.Handle("PUT /default", api.Health{})
	_ = http.ListenAndServe(":8080", api.Logging(mux))
}
