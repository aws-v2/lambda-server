package handlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/database"
	"lambda/internal/utils/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *LambdaHandlers) RegisterFunction(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.register"),
		zap.String(logger.F.Domain, "lambda"),
	)

	var req dto.RegisterFunctionRequest
	if err := c.ShouldBind(&req); err != nil {
		log.Warn("failed to bind register request",
			zap.String(logger.F.ErrorKind, "invalid_request"),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	uIDStr, _ := userID.(string)

	log = log.With(
		zap.String("function_name", req.Name),
		zap.String("user_id", uIDStr),
	)

	var artifactPath string

	if req.Execution.Kind == "image" {
		log.Info("registering image-based function",
			zap.String("image", req.Image),
		)
	} else {
		file, err := c.FormFile("file")
		if err != nil {
			log.Warn("missing file for non-image function",
				zap.String(logger.F.ErrorKind, "invalid_request"),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "binary file is required for non-image functions"})
			return
		}

		openedFile, err := file.Open()
		if err != nil {
			log.Error("failed to open uploaded file",
				zap.String(logger.F.ErrorKind, "file_open_error"),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
			return
		}
		defer openedFile.Close()

		ext := filepath.Ext(file.Filename)
		isZip := ext == ".zip" ||
			file.Header.Get("Content-Type") == "application/zip" ||
			file.Header.Get("Content-Type") == "application/x-zip-compressed"

		if isZip {
			artifactPath, err = h.Storage.SaveFunctionZip(req.Name, openedFile, file.Size)
			if err != nil {
				log.Error("failed to save function zip",
					zap.String(logger.F.ErrorKind, "storage_error"),
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save function zip"})
				return
			}
			log.Info("function zip saved", zap.String("artifact_path", artifactPath))
		} else {
			artifactPath, err = h.Storage.SaveFunctionBinary(req.Name, openedFile)
			if err != nil {
				log.Error("failed to save function binary",
					zap.String(logger.F.ErrorKind, "storage_error"),
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save function binary"})
				return
			}
			log.Info("function binary saved", zap.String("artifact_path", artifactPath))
		}
	}

	if req.Type == "" {
		req.Type = "lambda"
	}

	if len(req.Execution.Command) == 1 {
		cmdStr := req.Execution.Command[0]
		if strings.HasPrefix(cmdStr, "[") && strings.HasSuffix(cmdStr, "]") {
			var parsed []string
			if err := json.Unmarshal([]byte(cmdStr), &parsed); err == nil {
				req.Execution.Command = parsed
			}
		}
	}

	if len(req.Env) == 0 {
		rawEnv := c.PostForm("env")
		if rawEnv != "" && strings.HasPrefix(rawEnv, "{") && strings.HasSuffix(rawEnv, "}") {
			var parsed map[string]string
			if err := json.Unmarshal([]byte(rawEnv), &parsed); err == nil {
				req.Env = parsed
			}
		}
	}

	if req.Execution.Kind == "" {
		req.Execution.Kind = "binary"
	}

	if req.Image == "" {
		switch req.Execution.Kind {
		case "python":
			req.Image = "python:3.10-slim"
		case "node", "nodejs":
			req.Image = "node:18-alpine"
		case "java":
			req.Image = "amazoncorretto:17"
		default:
			req.Image = "golang:1.22-alpine"
		}
	}

	if req.Resources.CPU <= 0 {
		req.Resources.CPU = 1
	}
	if req.Resources.Memory <= 0 {
		req.Resources.Memory = 128
	}

	arn := h.GenerateArn(uIDStr, req.Name)

	err := h.DB.SaveFunction(database.Function{
		Name:   req.Name,
		ARN:    arn,
		UserID: uIDStr,
		Type:   req.Type,
		Image:  req.Image,
		Execution: database.ExecutionDetails{
			Kind:    req.Execution.Kind,
			Path:    artifactPath,
			Command: nil,
		},
		Resources: database.ResourceDetails{
			CPU:    req.Resources.CPU,
			Memory: req.Resources.Memory,
		},
		Env:         nil,
		TimeoutMS:   req.TimeoutMS,
		Description: req.Description,
	})
	if err != nil {
		log.Error("failed to save function metadata",
			zap.String(logger.F.ErrorKind, "db_write_error"),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save function metadata"})
		return
	}

	log.Info("function registered successfully", zap.String("function_arn", arn))

	c.JSON(http.StatusOK, gin.H{
		"message":       "function registered successfully",
		"name":          req.Name,
		"artifact_path": artifactPath,
	})
}

func (h *LambdaHandlers) ListFunctions(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.list"),
		zap.String(logger.F.Domain, "lambda"),
	)

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	functions, err := h.DB.ListFunctionsByUser(userIDStr)
	if err != nil {
		log.Error("failed to list functions",
			zap.String(logger.F.ErrorKind, "db_read_error"),
			zap.String("user_id", userIDStr),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch functions"})
		return
	}

	log.Info("functions retrieved successfully",
		zap.String("user_id", userIDStr),
		zap.Int("count", len(functions)),
	)

	c.JSON(http.StatusOK, functions)
}

func (h *LambdaHandlers) GetFunction(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.get"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := h.ResolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	log = log.With(
		zap.String("function_identifier", identifier),
		zap.String("user_id", userIDStr),
	)

	fn, err := h.ResolveFunction(identifier, userIDStr)
	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	log.Info("function retrieved successfully", zap.String("function_name", fn.Name))

	c.JSON(http.StatusOK, fn)
}

func (h *LambdaHandlers) GetCode(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.get_code"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := h.ResolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	log = log.With(
		zap.String("function_identifier", identifier),
		zap.String("user_id", userIDStr),
	)

	fn, err := h.ResolveFunction(identifier, userIDStr)
	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	content, err := h.Storage.ReadFunctionFile(fn.Name, "handler")
	if err != nil {
		log.Warn("code artifact not found",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.String("function_name", fn.Name),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "code artifact not found"})
		return
	}

	log.Info("function code retrieved", zap.String("function_name", fn.Name))

	c.Data(http.StatusOK, "text/plain", content)
}