package application

import (
	"fmt"
	"net/http"

	"lambda/internal/domain/dto"
	"lambda/internal/domain/models"
	"lambda/internal/utils"

	"log"

	"github.com/gin-gonic/gin"

	"lambda/internal/infrastructure/auth"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/storage"
)

type ConfigService struct {
	DB                *database.DB
	Storage           *storage.Storage
	Resolver          *auth.ApiKeyResolver
	Region            string
	NatsClient        *event.NatsClient
	ResolveFunction   func(identifier, userID string) (*models.Function, error)
	ResolveIdentifier func(c *gin.Context) string
}

func NewConfigService(db *database.DB, storage *storage.Storage, resolver *auth.ApiKeyResolver, region string, natsClient *event.NatsClient) *ConfigService {
	return &ConfigService{DB: db, Storage: storage, Resolver: resolver, Region: region, NatsClient: natsClient}
}

func (h *ConfigService) UpdateConfig(c *gin.Context, req dto.UpdateFunctionConfigRequest) error {

	fn, err := h.DB.GetFunction(req.FunctionID, req.UserId)
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", req.RequestId, err.Error())
		return err
	}

	err = h.DB.UpdateFunctionConfig(fn.ID, req.UserId, req.Memory, req.Timeout, req.Description)
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", req.RequestId, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("config could not update successfully"))
		return err
	}

	return nil

}

func (h *ConfigService) GetCode(c *gin.Context, req dto.GetCodeRequest) ([]byte, error) {

	content, err := h.Storage.ReadFunctionFile(req.FunctionID, "handler")
	if err != nil {
		log.Printf("[Handler:CreateDatabase] Service call,  requestID %s, error %s", req.RequestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("Failed to read code artifacts"))
		return nil, err
	}

	return content, nil

}
