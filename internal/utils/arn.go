package utils

import (
	"errors"
	"fmt"
	"strings"
)

// LambdaARN represents a structured Serwin Lambda ARN
type LambdaARN struct {
	Partition    string
	Service      string
	Region       string
	AccountID    string
	ResourceType string
	ResourceID   string
}

// GenerateLambdaARN creates a new Lambda ARN string
func GenerateLambdaARN(region, accountID, functionName string) string {
	return fmt.Sprintf("arn:serw:lambda:%s:%s:function/%s", region, accountID, functionName)
}

// ParseLambdaARN parses a string into a LambdaARN struct
func ParseLambdaARN(arnStr string) (*LambdaARN, error) {
	parts := strings.Split(arnStr, ":")
	if len(parts) != 6 {
		return nil, errors.New("invalid ARN format: must have 6 colon-separated parts")
	}

	if parts[0] != "arn" {
		return nil, errors.New("invalid ARN: must start with 'arn'")
	}
	if parts[1] != "serw" {
		return nil, errors.New("invalid ARN: partition must be 'serw'")
	}
	if parts[2] != "lambda" {
		return nil, errors.New("invalid ARN: service must be 'lambda'")
	}

	resourceParts := strings.SplitN(parts[5], "/", 2)
	if len(resourceParts) != 2 || resourceParts[0] != "function" {
		return nil, errors.New("invalid ARN: resource type must be 'function'")
	}

	return &LambdaARN{
		Partition:    parts[1],
		Service:      parts[2],
		Region:       parts[3],
		AccountID:    parts[4],
		ResourceType: resourceParts[0],
		ResourceID:   resourceParts[1],
	}, nil
}

// ValidateLambdaARN checks if the ARN string is a valid Lambda ARN
func ValidateLambdaARN(arnStr string) error {
	_, err := ParseLambdaARN(arnStr)
	return err
}

// String returns the string representation of the LambdaARN
func (a *LambdaARN) String() string {
	return fmt.Sprintf("arn:%s:%s:%s:%s:%s/%s",
		a.Partition, a.Service, a.Region, a.AccountID, a.ResourceType, a.ResourceID)
}
