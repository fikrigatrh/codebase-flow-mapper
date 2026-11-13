//go:build wireinject

package main

import "example.com/wirestale/internal/app"

func InitApp() *app.App { return app.New("api") }
