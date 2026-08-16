// echo-example runs an Echo server with the kaka rate limiting middleware.
// Try: curl -i localhost:8084/  (repeat to see 429 with Retry-After)
package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/themarchrain/kaka/memory"
	echomiddleware "github.com/themarchrain/kaka/middleware/echo"
)

func main() {
	limiter := memory.NewTokenBucket(10, 1, memory.WithMaxKeys(10000), memory.WithKeyTTL(0))

	e := echo.New()
	e.Use(echomiddleware.NewLimiterMiddleware(echomiddleware.Config{
		Limiter: limiter,
	}))
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "hello from echo + kaka")
	})
	e.Logger.Fatal(e.Start(":8084"))
}
