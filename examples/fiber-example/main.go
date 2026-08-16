// fiber-example runs a Fiber server with the kaka rate limiting middleware.
// Try: curl -i localhost:8085/  (repeat to see 429 with Retry-After)
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/themarchrain/kaka/memory"
	fibermiddleware "github.com/themarchrain/kaka/middleware/fiber"
)

func main() {
	limiter := memory.NewTokenBucket(10, 1, memory.WithMaxKeys(10000), memory.WithKeyTTL(0))

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(fibermiddleware.NewLimiterMiddleware(fibermiddleware.Config{
		Limiter: limiter,
	}))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("hello from fiber + kaka")
	})
	if err := app.Listen(":8085"); err != nil {
		panic(err)
	}
}
