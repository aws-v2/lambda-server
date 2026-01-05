package application

import (
	"context"
	"fmt"
	"lambda/internal/domain"
	"time"

	"github.com/google/uuid"
)

type FunctionService struct {
	functionRepo domain.FunctionRepository
	codeStorage  domain.CodeStorage
}

func NewFunctionService(functionRepo domain.FunctionRepository, codeStorage domain.CodeStorage) *FunctionService {
	return &FunctionService{
		functionRepo: functionRepo,
		codeStorage:  codeStorage,
	}
}

func (s *FunctionService) DeployFunction(ctx context.Context, name, ownerID string, runtime domain.Runtime, code string) (*domain.Function, error) {
	if !runtime.IsValid() {
		return nil, fmt.Errorf("invalid runtime: %s", runtime)
	}

	if name == "" || code == "" {
		return nil, fmt.Errorf("name and code are required")
	}

	existing, _ := s.functionRepo.GetByName(ctx, name, ownerID)
	if existing != nil {
		return nil, fmt.Errorf("function with name %s already exists", name)
	}

	function := &domain.Function{
		ID:        uuid.New().String(),
		Name:      name,
		OwnerID:   ownerID,
		Runtime:   runtime,
		Code:      code,
		Status:    domain.FunctionStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		MemoryMB:  128,
		TimeoutMs: 30000,
	}

	if err := s.functionRepo.Save(ctx, function); err != nil {
		return nil, fmt.Errorf("failed to save function: %w", err)
	}

	if err := s.codeStorage.SaveCode(ctx, function.ID, code); err != nil {
		return nil, fmt.Errorf("failed to save code: %w", err)
	}

	return function, nil
}

func (s *FunctionService) GetFunction(ctx context.Context, name, ownerID string) (*domain.Function, error) {
	return s.functionRepo.GetByName(ctx, name, ownerID)
}

func (s *FunctionService) ListFunctions(ctx context.Context, ownerID string) ([]domain.Function, error) {
	return s.functionRepo.ListByOwner(ctx, ownerID)
}

func (s *FunctionService) DeleteFunction(ctx context.Context, name, ownerID string) error {
	function, err := s.functionRepo.GetByName(ctx, name, ownerID)
	if err != nil {
		return fmt.Errorf("function not found: %w", err)
	}

	if err := s.functionRepo.Delete(ctx, function.ID); err != nil {
		return fmt.Errorf("failed to delete function: %w", err)
	}

	if err := s.codeStorage.DeleteCode(ctx, function.ID); err != nil {
		return fmt.Errorf("failed to delete code: %w", err)
	}

	return nil
}
