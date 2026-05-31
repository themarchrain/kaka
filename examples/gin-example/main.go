package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/themarchrain/kaka/memory"
	ginmiddleware "github.com/themarchrain/kaka/middleware/gin"
)

func main() {
	// 创建限流器：容量 10，每秒补充 2 个令牌
	limiter := memory.NewTokenBucket(10, 2)

	r := gin.Default()

	// 应用限流中间件
	r.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
		Limiter: limiter,
		KeyFunc: func(c *gin.Context) string {
			return c.ClientIP() // 按 IP 限流
		},
		Headers: true,
	}))

	// 测试路由
	r.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "success",
		})
	})

	r.GET("/api/user/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": c.Param("id"),
			"message": "user info",
		})
	})

	log.Println("Server starting on :8080...")
	log.Println("Test: curl http://localhost:8080/api/test")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
