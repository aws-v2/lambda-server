package http

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine, handlers *LambdaHandlers) {
	v1 := router.Group("/api/v1/lambda")
	v1.Use(AuthMiddleware())
	{
		v1.POST("/invoke", handlers.Invoke)
		v1.POST("/functions", handlers.RegisterFunction)
		v1.GET("/functions", handlers.ListFunctions)
		v1.GET("/functions/:name", handlers.GetFunction)
		v1.GET("/functions/:name/code", handlers.GetCode)
		v1.PATCH("/functions/:name/code", handlers.UpdateCode)
		v1.GET("/functions/:name/metrics", handlers.GetMetrics)
		v1.PATCH("/functions/:name/config", handlers.UpdateConfig)
		v1.POST("/functions/:name/invoke", handlers.Invoke)
		v1.POST("/functions/:name/test", handlers.Invoke)
	}

	router.GET("/api/v1/lambda/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
