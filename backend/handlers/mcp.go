package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"ops-ledger/backend/models"
)

// MCPHandler handles MCP (Model Context Protocol) requests
type MCPHandler struct {
	DB  *sql.DB
	Hub *EventHub
}

// JSON-RPC types
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool definitions
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// MCP tools/call response types (spec version 2024-11-05)
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// Handle processes MCP JSON-RPC requests
func (h *MCPHandler) Handle(c echo.Context) error {
	var req JSONRPCRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &RPCError{Code: -32700, Message: "Parse error"},
		})
	}

	var result interface{}
	var rpcErr *RPCError

	switch req.Method {
	case "initialize":
		result = h.handleInitialize()
	case "tools/list":
		result = h.handleToolsList()
	case "tools/call":
		result, rpcErr = h.handleToolCall(c.Request().Context(), req.Params)
	default:
		rpcErr = &RPCError{Code: -32601, Message: "Method not found"}
	}

	return c.JSON(http.StatusOK, JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
		Error:   rpcErr,
	})
}

func (h *MCPHandler) handleInitialize() map[string]any {
	return map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "ops-ledger",
			"version": "1.0.0",
		},
	}
}

func (h *MCPHandler) handleToolsList() map[string]any {
	tools := []Tool{
		{
			Name:        "register_change",
			Description: "Report a change made to any system so it is logged in the ops ledger. Call this tool immediately after completing any action that modifies shared infrastructure, configuration, or software — including: deploying a new version of a service, applying Terraform/IaC plans, changing environment variables or secrets, modifying cloud resources (security groups, IAM policies, DNS, databases), updating Kubernetes manifests, changing CI/CD pipeline configuration, rotating API keys or certificates, or scaling resources. Use `type` to classify the change: choose `deployment` for new releases or rollbacks, `infrastructure` for cloud/IaC/Kubernetes/networking changes, and `configuration` for env vars, secrets, feature flags, or settings changes. Always call this tool proactively — do not wait to be asked.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"system":      map[string]any{"type": "string", "description": "Name of the system or component that was changed (e.g. 'api-service', 'terraform/aws-prod', 'postgres-primary', 'k8s/ingress')"},
					"type":        map[string]any{"type": "string", "description": "Category of change: `deployment` = new release, rollback, or container image update; `infrastructure` = cloud resource, IaC, Kubernetes, networking, or security change; `configuration` = environment variable, secret, feature flag, settings file, or CI/CD pipeline change"},
					"description": map[string]any{"type": "string", "description": "Human-readable summary of exactly what changed and why. Be specific: include version numbers, resource names, key names (not values), before/after values where relevant. Examples: 'Deployed v2.4.1 (was v2.3.9) to fix memory leak in worker', 'Set MAX_CONNECTIONS=100 (was 50) to handle load increase', 'Applied terraform plan: added sg-0abc123 ingress rule for port 443'"},
					"environment": map[string]any{"type": "string", "description": "Target environment: production, staging, development, etc. Strongly recommended — omit only when not applicable (e.g. a shared service with no environment distinction)"},
					"user":        map[string]any{"type": "string", "description": "Identity of the agent or person who made the change. For AI agents use a descriptive name like 'claude-code', 'github-actions', or 'terraform-cloud'. For human-initiated changes, use their username."},
					"timestamp":   map[string]any{"type": "string", "description": "RFC3339 timestamp of when the change was applied (e.g. '2025-01-15T10:30:00Z'). Defaults to now. Use an explicit timestamp if reporting a change that happened in the past."},
				},
				"required": []string{"system", "type", "description"},
			},
		},
		{
			Name:        "update_change",
			Description: "Correct or enrich an existing change record that was previously logged. Use this when a change was registered with incomplete or incorrect information — for example, to add the environment after it was omitted, fix a wrong type classification, or update the description with the actual outcome (e.g. if a deployment failed and was rolled back). Requires the `id` of the record to update.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":          map[string]any{"type": "integer", "description": "Numeric ID of the change record to correct, as returned by `register_change` or `list_changes`"},
					"system":      map[string]any{"type": "string", "description": "Name of the system or component that was changed (e.g. 'api-service', 'terraform/aws-prod', 'postgres-primary', 'k8s/ingress')"},
					"type":        map[string]any{"type": "string", "description": "Category of change: `deployment` = new release, rollback, or container image update; `infrastructure` = cloud resource, IaC, Kubernetes, networking, or security change; `configuration` = environment variable, secret, feature flag, settings file, or CI/CD pipeline change"},
					"description": map[string]any{"type": "string", "description": "Human-readable summary of exactly what changed and why. Be specific: include version numbers, resource names, key names (not values), before/after values where relevant. Examples: 'Deployed v2.4.1 (was v2.3.9) to fix memory leak in worker', 'Set MAX_CONNECTIONS=100 (was 50) to handle load increase', 'Applied terraform plan: added sg-0abc123 ingress rule for port 443'"},
					"environment": map[string]any{"type": "string", "description": "Target environment: production, staging, development, etc. Strongly recommended — omit only when not applicable (e.g. a shared service with no environment distinction)"},
					"user":        map[string]any{"type": "string", "description": "Identity of the agent or person who made the change. For AI agents use a descriptive name like 'claude-code', 'github-actions', or 'terraform-cloud'. For human-initiated changes, use their username."},
					"timestamp":   map[string]any{"type": "string", "description": "RFC3339 timestamp of when the change was applied (e.g. '2025-01-15T10:30:00Z'). Defaults to now. Use an explicit timestamp if reporting a change that happened in the past."},
				},
				"required": []string{"id", "system", "type", "description"},
			},
		},
		{
			Name:        "delete_change",
			Description: "Permanently remove a change record from the ops ledger. Use only to remove records logged in error (e.g. a dry-run was mistakenly reported as a real change, or a duplicate record was created). Do not use to hide or retract legitimate changes — prefer `update_change` to correct inaccurate records instead.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "integer", "description": "Numeric ID of the change record to delete, as returned by `register_change` or `list_changes`"},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "list_changes",
			Description: "Query the ops ledger for previously recorded changes. Use this to check what changes have already been logged (to avoid duplicates), to look up a change ID before updating or deleting it, or to audit recent activity for a specific system or environment. Returns results newest-first.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"system":      map[string]any{"type": "string", "description": "Filter to changes for a specific system (exact match, case-insensitive)"},
					"environment": map[string]any{"type": "string", "description": "Filter to changes in a specific environment (e.g. 'production', 'staging')"},
					"user":        map[string]any{"type": "string", "description": "Filter to changes made by a specific user or agent"},
					"type":        map[string]any{"type": "string", "description": "Filter by category: `deployment`, `infrastructure`, or `configuration`"},
					"search":      map[string]any{"type": "string", "description": "Full-text search across description, system name, user, and environment fields"},
					"limit":       map[string]any{"type": "integer", "description": "Maximum number of records to return (default 50, max 200)"},
					"offset":      map[string]any{"type": "integer", "description": "Number of records to skip for pagination (use with `limit`)"},
				},
			},
		},
	}

	return map[string]any{
		"tools": tools,
	}
}

func (h *MCPHandler) handleToolCall(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var callParams ToolCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid params"}
	}

	var data interface{}
	var rpcErr *RPCError

	switch callParams.Name {
	case "register_change":
		data, rpcErr = h.createChange(ctx, callParams.Arguments)
	case "update_change":
		data, rpcErr = h.updateChange(ctx, callParams.Arguments)
	case "delete_change":
		data, rpcErr = h.deleteChange(ctx, callParams.Arguments)
	case "list_changes":
		data, rpcErr = h.listChanges(ctx, callParams.Arguments)
	default:
		return nil, &RPCError{Code: -32601, Message: "Tool not found"}
	}

	if rpcErr != nil {
		// Tool execution error — route into result so the LLM can see and self-correct (MCP spec)
		return ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: rpcErr.Message}},
			IsError: true,
		}, nil
	}

	text, err := json.Marshal(data)
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: "Failed to serialize result"}
	}

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: string(text)}},
	}, nil
}

func (h *MCPHandler) createChange(ctx context.Context, args map[string]any) (interface{}, *RPCError) {
	system, _ := args["system"].(string)
	typeStr, _ := args["type"].(string)
	description, _ := args["description"].(string)

	if system == "" || typeStr == "" || description == "" {
		return nil, &RPCError{Code: -32602, Message: "system, type, and description are required"}
	}

	validTypes := map[string]bool{"infrastructure": true, "deployment": true, "configuration": true}
	if !validTypes[typeStr] {
		return nil, &RPCError{Code: -32602, Message: "type must be infrastructure, deployment, or configuration"}
	}

	var environment *string
	if env, ok := args["environment"].(string); ok && env != "" {
		environment = &env
	}

	var user *string
	if u, ok := args["user"].(string); ok && u != "" {
		user = &u
	}

	var ts *time.Time
	if tsStr, ok := args["timestamp"].(string); ok && tsStr != "" {
		parsed, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			return nil, &RPCError{Code: -32602, Message: "Invalid timestamp format, use RFC3339 (e.g. 2025-01-15T10:30:00Z)"}
		}
		ts = &parsed
	}

	change, err := models.CreateChange(h.DB, system, environment, user, typeStr, description, ts)
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: "Failed to create change: " + err.Error()}
	}

	detail := system + ": " + description
	_ = models.CreateAuditEntry(h.DB, "mcp", nil, "change.create", "change", uint64Ptr(change.ID), &detail, nil)

	if h.Hub != nil {
		h.Hub.Publish(SSEEvent{Type: "change.created", Data: change})
	}
	return change, nil
}

func (h *MCPHandler) updateChange(ctx context.Context, args map[string]any) (interface{}, *RPCError) {
	id := getIntArg(args, "id", 0)
	if id <= 0 {
		return nil, &RPCError{Code: -32602, Message: "id is required"}
	}

	system, _ := args["system"].(string)
	typeStr, _ := args["type"].(string)
	description, _ := args["description"].(string)

	if system == "" || typeStr == "" || description == "" {
		return nil, &RPCError{Code: -32602, Message: "system, type, and description are required"}
	}

	validTypes := map[string]bool{"infrastructure": true, "deployment": true, "configuration": true}
	if !validTypes[typeStr] {
		return nil, &RPCError{Code: -32602, Message: "type must be infrastructure, deployment, or configuration"}
	}

	var environment *string
	if env, ok := args["environment"].(string); ok && env != "" {
		environment = &env
	}

	var user *string
	if u, ok := args["user"].(string); ok && u != "" {
		user = &u
	}

	var ts *time.Time
	if tsStr, ok := args["timestamp"].(string); ok && tsStr != "" {
		parsed, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			return nil, &RPCError{Code: -32602, Message: "Invalid timestamp format, use RFC3339 (e.g. 2025-01-15T10:30:00Z)"}
		}
		ts = &parsed
	}

	change, err := models.UpdateChange(h.DB, uint64(id), system, environment, user, typeStr, description, ts)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &RPCError{Code: -32602, Message: "Change not found"}
		}
		return nil, &RPCError{Code: -32000, Message: "Failed to update change: " + err.Error()}
	}

	detail := system + ": " + description
	_ = models.CreateAuditEntry(h.DB, "mcp", nil, "change.update", "change", uint64Ptr(change.ID), &detail, nil)

	if h.Hub != nil {
		h.Hub.Publish(SSEEvent{Type: "change.updated", Data: change})
	}
	return change, nil
}

func (h *MCPHandler) deleteChange(ctx context.Context, args map[string]any) (interface{}, *RPCError) {
	id := getIntArg(args, "id", 0)
	if id <= 0 {
		return nil, &RPCError{Code: -32602, Message: "id is required"}
	}

	// Fetch for audit log details
	change, err := models.GetChangeByID(h.DB, uint64(id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &RPCError{Code: -32602, Message: "Change not found"}
		}
		return nil, &RPCError{Code: -32000, Message: "Failed to fetch change: " + err.Error()}
	}

	if err := models.DeleteChange(h.DB, uint64(id)); err != nil {
		return nil, &RPCError{Code: -32000, Message: "Failed to delete change: " + err.Error()}
	}

	detail := change.System + ": " + change.Description
	_ = models.CreateAuditEntry(h.DB, "mcp", nil, "change.delete", "change", uint64Ptr(uint64(id)), &detail, nil)

	if h.Hub != nil {
		h.Hub.Publish(SSEEvent{Type: "change.deleted", Data: DeletedPayload{ID: fmt.Sprintf("%d", id)}})
	}
	return map[string]string{"message": "Change deleted"}, nil
}

func (h *MCPHandler) listChanges(ctx context.Context, args map[string]any) (interface{}, *RPCError) {
	f := models.ChangeFilters{
		System:      getStringArg(args, "system"),
		Environment: getStringArg(args, "environment"),
		User:        getStringArg(args, "user"),
		Type:        getStringArg(args, "type"),
		Search:      getStringArg(args, "search"),
	}

	f.Limit = getIntArg(args, "limit", 50)
	f.Offset = getIntArg(args, "offset", 0)

	if f.Limit > 200 {
		f.Limit = 200
	}

	changes, total, err := models.ListChanges(h.DB, f)
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: "Failed to list changes: " + err.Error()}
	}

	return map[string]any{
		"changes": changes,
		"total":   total,
		"limit":   f.Limit,
		"offset":  f.Offset,
	}, nil
}

func getStringArg(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func getIntArg(args map[string]any, key string, defaultVal int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}
