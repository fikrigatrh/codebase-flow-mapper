package main

import (
	"os"

	"github.com/gin-gonic/gin"

	"example.com/ginbasic/internal/handler"
	"example.com/ginbasic/internal/mw"
)

func main() {
	r := gin.New()
	h := handler.New()
	r.GET("/health", h.Ping)
	r.Use(mw.Logger())
	r.GET("/ping", h.Ping)
	api := r.Group("/api", mw.Auth)
	api.Use(mw.Trace)
	v1 := api.Group("v1")
	{
		v1.POST("/tx", mw.RateLimit(10), h.CreateTx)
		v1.GET("/tx/:id", func(c *gin.Context) { c.String(200, c.Param("id")) })
	}
	r.Handle("DELETE", "/tx/:id", h.DeleteTx)
	r.Any("/any", h.Ping)
	r.GET(os.Args[0], h.Ping)
	_ = r.Run()
}
