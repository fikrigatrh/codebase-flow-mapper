package main

import fiberv3 "github.com/gofiber/fiber/v3"

// Fiber v3 is not supported (PRD §scope): its import must yield ROUTER_UNSUPPORTED.
func v3app() { _ = fiberv3.New() }
