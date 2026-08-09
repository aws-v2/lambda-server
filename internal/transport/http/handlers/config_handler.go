package handlers

import (
	"fmt"
	"io"
	"net/http"

	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/auth"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/storage"
	"lambda/internal/utils"

	"log"

	"github.com/gin-gonic/gin"
)

type ConfigHandler struct {
	DB                *database.DB
	Storage           *storage.Storage
	Resolver          *auth.ApiKeyResolver
	Region            string
	NatsClient        *event.NatsClient
	ResolveFunction   func(identifier, userID string) (*database.Function, error)
	ResolveIdentifier func(c *gin.Context) string
}

func NewConfigHandler(db *database.DB, storage *storage.Storage, resolver *auth.ApiKeyResolver, region string, natsClient *event.NatsClient) *ConfigHandler {
	return &ConfigHandler{DB: db, Storage: storage, Resolver: resolver, Region: region, NatsClient: natsClient}
}

func (h *ConfigHandler) UpdateConfig(c *gin.Context) {

	requestID := c.GetString("requestID")

	identifier := h.ResolveIdentifier(c)
	userIDStr := c.GetString("userId")

	var req dto.UpdateFunctionConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Handler:CreateDatabase] Payload unmarshal, bad request for requestID %s, with error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("Bad request"))
		return
	}

	fn, err := h.ResolveFunction(identifier, userIDStr)
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("config is not found"))
		return
	}

	err = h.DB.UpdateFunctionConfig(fn.Name, userIDStr, req.Memory, req.Timeout, req.Description)
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("config could not update successfully"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "config updated succesfully", gin.H{"id": identifier, "status": "UPDATED"})

}

func (h *ConfigHandler) UpdateCode(c *gin.Context) {

	requestID := c.GetString("requestID")

	identifier := h.ResolveIdentifier(c)
	userID, _ := c.Get("userId")
	userIDStr, _ := userID.(string)

	fn, err := h.ResolveFunction(identifier, userIDStr)
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("config is not found"))
		return
	}

	content, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("Bad request"))

		return
	}

	err = h.Storage.WriteFunctionFile(fn.Name, "handler", content)
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("Failed to update code artifacts"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "code updated successfully", gin.H{"id": identifier, "status": "UPDATED"})
}

func (h *ConfigHandler) GetCode(c *gin.Context) {

	identifier := h.ResolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	requestID := c.GetString("requestID")

	fn, err := h.ResolveFunction(identifier, userIDStr)
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("function not found"))
		return
	}

	content, err := h.Storage.ReadFunctionFile(fn.Name, "handler")
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("Failed to read code artifacts"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "function code updated successfully", content)
}
