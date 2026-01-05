package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
)

type ValidationRequest struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
}

type ValidationResponse struct {
	Valid  bool   `json:"valid"`
	UserID string `json:"userId"`
}

func AuthMiddleware(nc *nats.Conn) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("x-api-key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing api key"})
			return
		}

		parts := strings.Split(apiKey, ":")
		if len(parts) != 2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key format"})
			return
		}

		req := ValidationRequest{
			AccessKeyID:     parts[0],
			SecretAccessKey: parts[1],
		}

		data, _ := json.Marshal(req)
		msg, err := nc.Request("iam.auth.validate", data, 2*time.Second)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "auth service unavailable"})
			return
		}

		var resp ValidationResponse
		json.Unmarshal(msg.Data, &resp)

		if !resp.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		c.Set("userId", resp.UserID)
		c.Next()
	}
}
