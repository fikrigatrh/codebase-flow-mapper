package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"example.com/fiberapp/internal/handler"
	"example.com/fiberapp/internal/mw"
)

func main() {
	app := fiber.New()
	app.Get("/health", handler.Health)
	app.Use(logger.New())
	app.Use("/api", mw.Auth)
	api := app.Group("/api", mw.Trace)
	v1 := api.Group("v1")
	v1.Get("/users/:id", handler.GetUser)
	v1.Post("/users", mw.RateLimit(10), handler.CreateUser)
	v1.All("/any", handler.Health)
	v1.Add("PATCH", "/users/:id", handler.CreateUser)
	app.Get("/apix", handler.Health)
	app.Get("/public", handler.Health)
	api.Route("/admin", func(r fiber.Router) {
		r.Delete("/users/:id", handler.DeleteUser)
	})
	app.Get("/a", handler.Health).Get("/b", func(c *fiber.Ctx) error { return c.SendString("b") })
	handler.Register(app)
	_ = app.Listen(":3000")
}
