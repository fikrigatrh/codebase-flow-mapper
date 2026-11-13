//go:build wireinject

package main

import "example.com/wiremissing/internal/app"

func InitApp() *app.App { return app.New() }
