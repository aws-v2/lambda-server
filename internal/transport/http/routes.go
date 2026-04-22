package http

import (
	"lambda/internal/transport/http/handlers"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(
	router *gin.Engine,
	handlers *handlers.LambdaHandlers, 
	invokeHandlers *handlers.InvokeHandler, 
	configHandlers *handlers.ConfigHandler, 
	metricsHandlers *handlers.MetricHandler, 
	policyHandlers *handlers.PolicyLambdaHandlers,

	docsHandlers *handlers.DocsHandler,	 

) {
	v1 := router.Group("/api/v1/lambda")
	{
		// Generic invoke (no function name in path)
		v1.POST("/invoke", invokeHandlers.Invoke)

		// Function CRUD
		v1.POST("/functions", handlers.RegisterFunction)
		v1.GET("/functions", handlers.ListFunctions)
		v1.GET("/functions/:name", handlers.GetFunction)
		v1.GET("/functions/:name/code", configHandlers.GetCode)
		v1.PATCH("/functions/:name/code", configHandlers.UpdateCode)
		v1.GET("/functions/:name/metrics", metricsHandlers.GetMetrics)
		v1.PATCH("/functions/:name/config", configHandlers.UpdateConfig)
		v1.POST("/functions/:name/invoke", invokeHandlers.Invoke)
		v1.POST("/functions/:name/test", invokeHandlers.Invoke)

		// ARN-based routing
		v1.GET("/functions/arn/*arn", handlers.ArnRouter)
		v1.POST("/functions/arn/*arn", handlers.ArnRouter)
		v1.PATCH("/functions/arn/*arn", handlers.ArnRouter)
	}

	v1.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Scaling policies
	scalingPolicies := v1.Group("")
	{
		scalingPolicies.GET("/policies", policyHandlers.ListLambdaScalingPolicies)
		scalingPolicies.POST("/:functionId/policies", policyHandlers.CreateLambdaScalingPolicy)
		scalingPolicies.PUT("/:functionId/policies", policyHandlers.UpdateLambdaScalingPolicy)
		scalingPolicies.DELETE("/:functionId/policies", policyHandlers.DeleteLambdaScalingPolicy)
	}

	// Docs
	docs := v1.Group("/docs")
	{
		docs.GET("", docsHandlers.GetPublicManifest)
		docs.GET("/:slug", docsHandlers.GetPublicDoc)
	}

	internalDocs := v1.Group("/internal/docs")
	{
		internalDocs.GET("", docsHandlers.GetInternalManifest)
		internalDocs.GET("/:slug", docsHandlers.GetInternalDoc)
	}
}