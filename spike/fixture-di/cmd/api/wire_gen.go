//go:build !wireinject

package main

import (
	"example.com/fixturedi/internal/handler"
	"example.com/fixturedi/internal/repo"
	"example.com/fixturedi/internal/service"
)

func initApp() *handler.TxHandler {
	r := repo.NewPg()
	s := service.New(r)
	return handler.New(s)
}
