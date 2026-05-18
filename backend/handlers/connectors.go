package handlers

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	mw "ops-ledger/backend/middleware"
	"ops-ledger/backend/models"
)

type ConnectorHandler struct {
	DB  *sql.DB
	Hub *EventHub
}

func (h *ConnectorHandler) requireAdmin(c echo.Context) (*mw.JWTClaims, uint64, error) {
	claims := c.Get("claims").(*mw.JWTClaims)
	if claims.Role != "admin" {
		return nil, 0, c.JSON(http.StatusForbidden, map[string]string{"error": "Admin access required"})
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return nil, 0, c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token claims"})
	}
	id, err := strconv.ParseUint(sub, 10, 64)
	if err != nil {
		return nil, 0, c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token claims"})
	}
	return claims, id, nil
}

// List returns all connectors (base info only, no secrets or tokens).
func (h *ConnectorHandler) List(c echo.Context) error {
	if _, _, err := h.requireAdmin(c); err != nil {
		return nil
	}
	connectors, err := models.ListConnectors(h.DB)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list connectors"})
	}
	if connectors == nil {
		connectors = []models.Connector{}
	}
	return c.JSON(http.StatusOK, connectors)
}

type createJiraConnectorRequest struct {
	Name     string          `json:"name"`
	JiraURL  string          `json:"jira_url"`
	APIToken string          `json:"api_token"`
	Mapping  json.RawMessage `json:"mapping"`
}

// Create adds a new Jira connector.
func (h *ConnectorHandler) Create(c echo.Context) error {
	_, userID, err := h.requireAdmin(c)
	if err != nil {
		return nil
	}

	var req createJiraConnectorRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if req.Name == "" || req.JiraURL == "" || req.APIToken == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name, jira_url, and api_token are required"})
	}

	mapping := req.Mapping
	if len(mapping) == 0 {
		mapping = json.RawMessage(`{"type_map":{},"environment_label_prefix":"env:"}`)
	}

	jc, err := models.CreateJiraConnector(h.DB, req.Name, req.JiraURL, req.APIToken, mapping, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create connector"})
	}

	details := strPtr("type=jira name=" + jc.Name)
	auditLog(h.DB, c, "connector.create", "connector", nil, strPtr(jc.ID), details)

	return c.JSON(http.StatusCreated, jc)
}

type updateJiraConnectorRequest struct {
	Name     string          `json:"name"`
	JiraURL  string          `json:"jira_url"`
	APIToken string          `json:"api_token"` // empty means keep existing
	Mapping  json.RawMessage `json:"mapping"`
	Enabled  bool            `json:"enabled"`
}

// Update modifies an existing Jira connector. An empty api_token preserves the stored value.
func (h *ConnectorHandler) Update(c echo.Context) error {
	if _, _, err := h.requireAdmin(c); err != nil {
		return nil
	}

	id := c.Param("id")
	var req updateJiraConnectorRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if req.Name == "" || req.JiraURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name and jira_url are required"})
	}

	mapping := req.Mapping
	if len(mapping) == 0 {
		mapping = json.RawMessage(`{"type_map":{},"environment_label_prefix":"env:"}`)
	}

	if err := models.UpdateJiraConnector(h.DB, id, req.Name, req.JiraURL, req.APIToken, mapping, req.Enabled); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Connector not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update connector"})
	}

	details := strPtr("id=" + id)
	auditLog(h.DB, c, "connector.update", "connector", nil, strPtr(id), details)

	jc, err := models.GetJiraConnectorByID(h.DB, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to load connector"})
	}
	return c.JSON(http.StatusOK, jc)
}

// Delete removes a connector and its type-specific config (CASCADE).
func (h *ConnectorHandler) Delete(c echo.Context) error {
	if _, _, err := h.requireAdmin(c); err != nil {
		return nil
	}

	id := c.Param("id")
	if err := models.DeleteConnector(h.DB, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete connector"})
	}

	details := strPtr("id=" + id)
	auditLog(h.DB, c, "connector.delete", "connector", nil, strPtr(id), details)

	return c.JSON(http.StatusOK, map[string]string{"message": "Connector deleted"})
}

// jiraWebhookPayload contains the fields we parse from a Jira webhook event.
type jiraWebhookPayload struct {
	WebhookEvent string `json:"webhookEvent"`
	Issue        struct {
		Key    string `json:"key"`
		Fields struct {
			Summary   string   `json:"summary"`
			IssueType struct { Name string `json:"name"` } `json:"issuetype"`
			Project   struct { Key string `json:"key"` } `json:"project"`
			Labels    []string `json:"labels"`
			Reporter  struct { EmailAddress string `json:"emailAddress"` } `json:"reporter"`
		} `json:"fields"`
	} `json:"issue"`
}

// Webhook receives Jira webhook events and converts matching issues into changes.
func (h *ConnectorHandler) Webhook(c echo.Context) error {
	id := c.Param("id")

	jc, err := models.GetJiraConnectorByID(h.DB, id)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Connector not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Internal error"})
	}
	if !jc.Enabled {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Connector is disabled"})
	}

	incoming := c.Request().Header.Get("X-Connector-Secret")
	if subtle.ConstantTimeCompare([]byte(incoming), []byte(jc.Secret)) != 1 {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Invalid secret"})
	}

	var payload jiraWebhookPayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
	}

	if payload.WebhookEvent != "jira:issue_created" && payload.WebhookEvent != "jira:issue_updated" {
		return c.JSON(http.StatusOK, map[string]string{"message": "Event ignored"})
	}

	var mapping models.JiraMapping
	if err := json.Unmarshal(jc.Mapping, &mapping); err != nil {
		mapping = models.JiraMapping{EnvironmentLabelPrefix: "env:"}
	}

	system := payload.Issue.Fields.Project.Key
	description := payload.Issue.Fields.Summary + " (" + payload.Issue.Key + ")"

	changeType := mapping.TypeMap[payload.Issue.Fields.IssueType.Name]
	if changeType == "" {
		changeType = "configuration"
	}

	var environment *string
	prefix := mapping.EnvironmentLabelPrefix
	for _, label := range payload.Issue.Fields.Labels {
		if prefix != "" && strings.HasPrefix(label, prefix) {
			env := strings.TrimPrefix(label, prefix)
			environment = &env
			break
		}
	}

	reporter := payload.Issue.Fields.Reporter.EmailAddress
	if reporter == "" {
		reporter = "jira"
	}
	user := "jira:" + reporter

	now := time.Now()
	change, err := models.CreateChange(h.DB, system, environment, &user, changeType, description, "executed", &now)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create change"})
	}

	if h.Hub != nil {
		h.Hub.Publish(SSEEvent{Type: "change.created", Data: change})
	}

	return c.JSON(http.StatusOK, change)
}
