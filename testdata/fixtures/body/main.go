package main

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"example.com/body/internal/handler"
	"example.com/body/internal/service"
)

func main() {
	h := handler.New(service.New())
	r := gin.New()
	r.POST("/v1/topup", h.Topup)
	r.GET("/v1/ping", h.Ping)
	http.HandleFunc("/legacy", handler.Legacy)
	_ = r.Run()
}
