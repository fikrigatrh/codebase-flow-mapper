package handler

import "github.com/gofiber/fiber/v2"

func Health(c *fiber.Ctx) error     { return c.SendStatus(fiber.StatusOK) }
func GetUser(c *fiber.Ctx) error    { return c.JSON(fiber.Map{"id": c.Params("id")}) }
func CreateUser(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }
func DeleteUser(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) }

// Register receives the app through the fiber.Router interface (rule 4).
func Register(r fiber.Router) {
	r.Get("/v2/users", GetUser)
}
