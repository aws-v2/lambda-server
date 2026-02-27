package dto

type PolicyDocument struct {
	Version   string      `json:"Version"`
	Statement []Statement `json:"Statement"`
}

type Statement struct {
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource []string `json:"Resource"`
}

type PolicyRequest struct {
	AccountID      string         `json:"account_id" binding:"required"`
	PrincipalArn   string         `json:"principal_arn" binding:"required"`
	PolicyName     string         `json:"policy_name" binding:"required"`
	PolicyDocument PolicyDocument `json:"policy_document" binding:"required"`
}

type PolicyEvent struct {
	RequestID      string         `json:"request_id"`
	AccountID      string         `json:"account_id,omitempty"`
	PrincipalArn   string         `json:"principal_arn,omitempty"`
	PolicyName     string         `json:"policy_name,omitempty"`
	PolicyDocument *PolicyDocument `json:"policy_document,omitempty"`
	PolicyID       string         `json:"policy_id,omitempty"`
	Error          string         `json:"error,omitempty"`
}
