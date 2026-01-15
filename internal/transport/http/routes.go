package http

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine, handlers *LambdaHandlers) {
	router.POST("/api/v1/lambda/invoke", handlers.Invoke)
	router.POST("/api/v1/lambda/functions", handlers.RegisterFunction)


	router.GET("/api/v1/lambda/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
