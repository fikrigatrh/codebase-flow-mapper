package mw

import "github.com/gofiber/fiber/v2"

func Auth(c *fiber.Ctx) error  { return c.Next() }
func Trace(c *fiber.Ctx) error { return c.Next() }

func RateLimit(n int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if n <= 0 {
			return fiber.ErrTooManyRequests
		}
		return c.Next()
	}
}
