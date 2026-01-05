package application

import (
	"context"
	"encoding/json"
	"fmt"
	"lambda/internal/domain"
	"time"

	"github.com/google/uuid"
)

type InvocationService struct {
	functionRepo   domain.FunctionRepository
	invocationRepo domain.InvocationRepository
	executors      map[domain.Runtime]domain.RuntimeExecutor
	eventPublisher domain.EventPublisher
}

func NewInvocationService(
	functionRepo domain.FunctionRepository,
	invocationRepo domain.InvocationRepository,
	executors map[domain.Runtime]domain.RuntimeExecutor,
	eventPublisher domain.EventPublisher,
) *InvocationService {
	return &InvocationService{
		functionRepo:   functionRepo,
		invocationRepo: invocationRepo,
		executors:      executors,
		eventPublisher: eventPublisher,
	}
}

func (s *InvocationService) InvokeFunction(ctx context.Context, name, ownerID string, payload map[string]interface{}) (interface{}, error) {
	function, err := s.functionRepo.GetByName(ctx, name, ownerID)
	if err != nil {
		return nil, fmt.Errorf("function not found: %w", err)
	}

	if function.Status != domain.FunctionStatusActive {
		return nil, fmt.Errorf("function is not active: %s", function.Status)
	}

	executor, ok := s.executors[function.Runtime]
	if !ok {
		return nil, fmt.Errorf("no executor found for runtime: %s", function.Runtime)
	}

	payloadJSON, _ := json.Marshal(payload)
	invocation := &domain.Invocation{
		ID:           uuid.New().String(),
		FunctionID:   function.ID,
		FunctionName: function.Name,
		Status:       domain.InvocationStatusRunning,
		Payload:      string(payloadJSON),
		StartedAt:    time.Now(),
	}

	if err := s.invocationRepo.Save(ctx, invocation); err != nil {
		return nil, fmt.Errorf("failed to save invocation: %w", err)
	}

	start := time.Now()
	result, execErr := executor.Execute(ctx, function, payload)
	duration := time.Since(start)

	completedAt := time.Now()
	invocation.CompletedAt = &completedAt
	invocation.DurationMs = duration.Milliseconds()

	if execErr != nil {
		invocation.Status = domain.InvocationStatusFailed
		invocation.Error = execErr.Error()
	} else {
		invocation.Status = domain.InvocationStatusSuccess
		resultJSON, _ := json.Marshal(result)
		invocation.Result = string(resultJSON)
	}

	if err := s.invocationRepo.Update(ctx, invocation); err != nil {
		return nil, fmt.Errorf("failed to update invocation: %w", err)
	}

	if s.eventPublisher != nil {
		s.eventPublisher.PublishInvocationEvent(ctx, invocation)
	}

	if execErr != nil {
		return nil, fmt.Errorf("execution failed: %w", execErr)
	}

	return result, nil
}

func (s *InvocationService) GetInvocationHistory(ctx context.Context, functionName, ownerID string, limit int) ([]domain.Invocation, error) {
	function, err := s.functionRepo.GetByName(ctx, functionName, ownerID)
	if err != nil {
		return nil, fmt.Errorf("function not found: %w", err)
	}

	return s.invocationRepo.ListByFunction(ctx, function.ID, limit)
}
