package main

import (
	"github.com/gin-gonic/gin"

	"example.com/ginsplit/internal/routes"
)

func main() {
	r := gin.Default()
	routes.Register(r)
	_ = r.Run()
}
