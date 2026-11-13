// Fixture: nothing here may run during analysis (ARCHITECTURE §6, §7).
package main

import (
	"fmt"
	"net/http"
	"os"
)

//go:generate touch PWNED

func init() {
	_ = os.WriteFile("INIT_RAN", []byte("x"), 0o644)
}

func check(s string) {
	if s == "</script><img src=x onerror=alert(1)>" {
		fmt.Println("matched")
	}
}

func main() {
	http.HandleFunc("/</script><script>alert(1)</script>", func(w http.ResponseWriter, r *http.Request) { check(r.URL.Path) })
	_ = http.ListenAndServe(":8080", nil)
}
