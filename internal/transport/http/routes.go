package http

import (
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"lambda/internal/middleware"
)

func SetupRoutes(router *gin.Engine, handlers *LambdaHandlers, nc *nats.Conn) {
	router.Use(middleware.LoggingMiddleware())

	v1 := router.Group("/api/v1/lambda")
	v1.Use(middleware.AuthMiddleware(nc))
	{
		v1.POST("/functions", handlers.CreateFunction)
		v1.GET("/functions", handlers.ListFunctions)
		v1.POST("/functions/:name/invoke", handlers.InvokeFunction)
		v1.DELETE("/functions/:name", handlers.DeleteFunction)
	}
}
