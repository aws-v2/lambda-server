package http

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine, handlers *LambdaHandlers) {
	v1 := router.Group("/api/v1/lambda")
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
		v1.GET("/functions/arn/*arn", handlers.ArnRouter)
		v1.POST("/functions/arn/*arn", handlers.ArnRouter)
		v1.PATCH("/functions/arn/*arn", handlers.ArnRouter)
	}

	v1.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// ── Policy routes ──────────────────────────────────────────────────────────
	policies := v1.Group("/policies")
	{
		policies.POST("", handlers.CreatePolicy)
		policies.PUT("/:policy_id", handlers.UpdatePolicy)
		policies.DELETE("/:policy_id", handlers.DeletePolicy)
		policies.GET("/:principal_id", handlers.GetPolicy)
	}

	// ── Lambda Scaling Policy routes ───────────────────────────────────────────
	scalingPolicies := v1.Group("")
	{
		scalingPolicies.GET("/policies", handlers.ListLambdaScalingPolicies)
		scalingPolicies.POST("/:functionId/policies", handlers.CreateLambdaScalingPolicy)
		scalingPolicies.PUT("/:functionId/policies", handlers.UpdateLambdaScalingPolicy)
		scalingPolicies.DELETE("/:functionId/policies", handlers.DeleteLambdaScalingPolicy)
	}

	// ── Public documentation endpoints (no auth required) ─────────────────────
	v1.GET("/docs", handlers.GetManifest)
	v1.GET("/docs/:slug", handlers.GetDocBySlug)
}
