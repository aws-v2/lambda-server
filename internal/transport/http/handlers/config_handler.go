package handlers

import (
	"fmt"
	"net/http"

	"lambda/internal/application"
	"lambda/internal/domain/dto"
	"lambda/internal/utils"

	"log"

	"github.com/gin-gonic/gin"
)

type ConfigHandler struct {
	ConfigService *application.ConfigService
}

func NewConfigHandler(configService *application.ConfigService) *ConfigHandler {
	return &ConfigHandler{
		ConfigService: configService,
	}
}

func (h *ConfigHandler) UpdateConfig(c *gin.Context) {

	requestID := c.GetString("requestID")
	userIDStr := c.GetString("userId")

	var req dto.UpdateFunctionConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Handler:CreateDatabase] Payload unmarshal, bad request for requestID %s, with error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("Bad request"))
		return
	}
	req.UserId = userIDStr
	req.RequestId = requestID

	err := h.ConfigService.UpdateConfig(c, req)
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("config could not update successfully"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "config updated succesfully", gin.H{"status": "UPDATED"})

}

func (h *ConfigHandler) GetCode(c *gin.Context) {

	userID := c.GetString("userId")
	requestId := c.GetString("requestId")

	requestID := c.GetString("requestID")
	var req dto.GetCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Handler:CreateDatabase] Payload unmarshal, bad request for requestID %s, with error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("Bad request"))
		return
	}

	req.UserID = userID
	req.RequestID = requestId

	content, err := h.ConfigService.GetCode(c, req)
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("Failed to read code artifacts"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "function code updated successfully", content)
}
