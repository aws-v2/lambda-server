package http

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine, handlers *LambdaHandlers) {
	v1 := router.Group("/api/v1/lambda")
	v1.Use(AuthMiddleware(handlers.Resolver))
	{
		// ── Name-based routes (existing, unchanged) ──────────────────────────────
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

		// ── ARN-based routes ──────────────────────────────────────────────────────
		// ARNs contain colons (arn:serwin:lambda:region:account:function:name) so they
		// cannot fit inside a :param segment. Gin's *param wildcard captures everything
		// after the prefix, including slashes and colons.
		//
		// A SINGLE wildcard route per HTTP method handles all sub-paths by inspecting
		// the captured suffix inside the handler (arnRouter). This avoids Gin's
		// "wildcard route conflicts" restriction.
		//
		// Examples:
		//   GET  /api/v1/lambda/functions/arn/arn:serwin:lambda:us-east-1:123:function:myFn
		//   GET  /api/v1/lambda/functions/arn/arn:serwin:lambda:us-east-1:123:function:myFn/metrics
		//   POST /api/v1/lambda/functions/arn/arn:serwin:lambda:us-east-1:123:function:myFn/invoke
		//   POST /api/v1/lambda/functions/arn/arn:serwin:lambda:us-east-1:123:function:myFn/test
		//   PATCH /api/v1/lambda/functions/arn/arn:serwin:lambda:us-east-1:123:function:myFn/config
		//   PATCH /api/v1/lambda/functions/arn/arn:serwin:lambda:us-east-1:123:function:myFn/code
		v1.GET("/functions/arn/*arn", handlers.ArnRouter)
		v1.POST("/functions/arn/*arn", handlers.ArnRouter)
		v1.PATCH("/functions/arn/*arn", handlers.ArnRouter)
	}

	router.GET("/api/v1/lambda/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
