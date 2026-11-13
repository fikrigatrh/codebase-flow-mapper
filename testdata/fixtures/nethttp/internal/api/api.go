package api

import "net/http"

type Users struct{}

func (*Users) Get(w http.ResponseWriter, r *http.Request)    {}
func (*Users) Delete(w http.ResponseWriter, r *http.Request) {}

func CreateUser(w http.ResponseWriter, r *http.Request) {}
func Static(w http.ResponseWriter, r *http.Request)     {}

type Health struct{}

func (Health) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r) })
}

func RequireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Role") != role {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Mount receives the mux as a parameter (rule 4); net/http has no prefixes.
func Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /v2/users", CreateUser)
}
