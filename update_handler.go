package main

import (
"bytes"
"io/ioutil"
"regexp"
"strings"
)

func main() {
content, err := ioutil.ReadFile("/home/martin/Desktop/aws-v7/asg/metrics-server/metrics-gateway/internal/transport/scaling_policy_nats_handler.go")
if err != nil {
ic(err)
}

// 1. Add Token string `json:"token"` to all Event/Request structs before CorrelationID (or just anywhere)
c := string(content)

c = regexp.MustCompile(`CorrelationID string\s*\x60json:"correlation_id"\x60`).ReplaceAllString(c, "CorrelationID string `json:\"correlation_id\"`\n\tToken         string `json:\"token\"`")

// 2. Add JWT Helper function
helper := `
import (
"context"
"encoding/json"
"encoding/base64"
"strings"
"fmt"
`
c = strings.Replace(c, "import (\n\t\"context\"\n\t\"encoding/json\"\n\t\"fmt\"\n", helper, 1)

helperFunc := `

func extractTenantIDFromToken(tokenString string) string {
if tokenString == "" {
 ""
}
parts := strings.Split(tokenString, ".")
if len(parts) != 3 {
 ""
}
payload, err := base64.RawURLEncoding.DecodeString(parts[1])
if err != nil {
 ""
}
var claims map[string]interface{}
if err := json.Unmarshal(payload, &claims); err != nil {
 ""
}
if userID, ok := claims["userId"].(string); ok {
 userID
}
if userID, ok := claims["user_id"].(string); ok {
 userID
}
if sub, ok := claims["sub"].(string); ok {
 sub
}
return ""
}

// ── Handlers`
c = strings.Replace(c, "// ── Handlers", helperFunc, 1)

// 3. For all handlers, replace event.TenantID validation with token extraction
// For EC2 Create
c = strings.Replace(c, "req := event.Policy\n\treq.TenantID = event.TenantID", "event.TenantID = extractTenantIDFromToken(event.Token)\n\treq := event.Policy\n\treq.TenantID = event.TenantID", 1)
// For EC2 Update
c = strings.Replace(c, "if event.TenantID == \"\"", "event.TenantID = extractTenantIDFromToken(event.Token)\n\n\tif event.TenantID == \"\"", 1)

// For EC2 Delete
c = strings.Replace(c, "if event.TenantID == \"\"", "event.TenantID = extractTenantIDFromToken(event.Token)\n\n\tif event.TenantID == \"\"", 1)
// For EC2 List
c = strings.Replace(c, "var req ScalingPolicyListRequest\n\tif err := json.Unmarshal", "var req ScalingPolicyListRequest\n\tif err := json.Unmarshal", 1)
c = strings.Replace(c, "policies, err := h.repo.ListEC2ScalingPolicies(ctx, req.TenantID)", "req.TenantID = extractTenantIDFromToken(req.Token)\n\tpolicies, err := h.repo.ListEC2ScalingPolicies(ctx, req.TenantID)", 1)

// For RDS Create
c = strings.Replace(c, "req := event.Policy\n\treq.TenantID = event.TenantID", "event.TenantID = extractTenantIDFromToken(event.Token)\n\treq := event.Policy\n\treq.TenantID = event.TenantID", 1)
// For RDS Update
c = strings.Replace(c, "if event.TenantID == \"\"", "event.TenantID = extractTenantIDFromToken(event.Token)\n\n\tif event.TenantID == \"\"", 1)
// For RDS Delete
c = strings.Replace(c, "if event.TenantID == \"\"", "event.TenantID = extractTenantIDFromToken(event.Token)\n\n\tif event.TenantID == \"\"", 1)
// For RDS List
c = strings.Replace(c, "policies, err := h.repo.ListRDSScalingPolicies(ctx, req.TenantID)", "req.TenantID = extractTenantIDFromToken(req.Token)\n\tpolicies, err := h.repo.ListRDSScalingPolicies(ctx, req.TenantID)", 1)

// For Lambda Create
c = strings.Replace(c, "req := event.Policy\n\treq.TenantID = event.TenantID", "event.TenantID = extractTenantIDFromToken(event.Token)\n\treq := event.Policy\n\treq.TenantID = event.TenantID", 1)
// For Lambda Update
c = strings.Replace(c, "if event.TenantID == \"\"", "event.TenantID = extractTenantIDFromToken(event.Token)\n\n\tif event.TenantID == \"\"", 1)
// For Lambda Delete
c = strings.Replace(c, "if event.TenantID == \"\"", "event.TenantID = extractTenantIDFromToken(event.Token)\n\n\tif event.TenantID == \"\"", 1)
// For Lambda List
c = strings.Replace(c, "policies, err := h.repo.ListLambdaScalingPolicies(ctx, req.TenantID)", "req.TenantID = extractTenantIDFromToken(req.Token)\n\tpolicies, err := h.repo.ListLambdaScalingPolicies(ctx, req.TenantID)", 1)


ioutil.WriteFile("/home/martin/Desktop/aws-v7/asg/metrics-server/metrics-gateway/internal/transport/scaling_policy_nats_handler.go", []byte(c), 0644)
}
