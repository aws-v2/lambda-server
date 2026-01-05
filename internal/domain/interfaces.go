package domain

import (
	"context"
)

type FunctionRepository interface {
	Save(ctx context.Context, function *Function) error
	GetByID(ctx context.Context, id string) (*Function, error)
	GetByName(ctx context.Context, name, ownerID string) (*Function, error)
	ListByOwner(ctx context.Context, ownerID string) ([]Function, error)
	Update(ctx context.Context, function *Function) error
	Delete(ctx context.Context, id string) error
}

type InvocationRepository interface {
	Save(ctx context.Context, invocation *Invocation) error
	GetByID(ctx context.Context, id string) (*Invocation, error)
	ListByFunction(ctx context.Context, functionID string, limit int) ([]Invocation, error)
	Update(ctx context.Context, invocation *Invocation) error
}

type CodeStorage interface {
	SaveCode(ctx context.Context, functionID, code string) error
	GetCode(ctx context.Context, functionID string) (string, error)
	DeleteCode(ctx context.Context, functionID string) error
}

type RuntimeExecutor interface {
	Execute(ctx context.Context, function *Function, payload map[string]interface{}) (interface{}, error)
	GetRuntime() Runtime
}

type EventPublisher interface {
	PublishInvocationEvent(ctx context.Context, invocation *Invocation) error
}
