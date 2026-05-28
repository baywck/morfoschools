package app

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

type aiModelInfo struct {
	ID string `json:"id"`
}

type aiProviderSettingsResponse struct {
	Scope           string        `json:"scope"`
	BaseURL         string        `json:"baseUrl"`
	HasAPIKey       bool          `json:"hasApiKey"`
	DefaultModel    string        `json:"defaultModel"`
	AvailableModels []aiModelInfo `json:"availableModels"`
	ChatbotModels   []aiModelInfo `json:"chatbotModels"`
	AllowedRoles    []string      `json:"allowedRoles,omitempty"`
	Enabled         bool          `json:"enabled"`
	UpdatedAt       string        `json:"updatedAt,omitempty"`
}

type aiProviderSettingsRequest struct {
	BaseURL       string   `json:"baseUrl"`
	APIKey        string   `json:"apiKey"`
	DefaultModel  string   `json:"defaultModel"`
	ChatbotModels []string `json:"chatbotModels"`
	AllowedRoles  []string `json:"allowedRoles"`
	Enabled       *bool    `json:"enabled"`
}

type resolvedAIProvider struct {
	Scope        string
	BaseURL      string
	APIKey       string
	DefaultModel string
	Models       []aiModelInfo
}

const maskedAIKey = "*********************Jks"

func (a *App) registerAIProviderSettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/ai/settings", a.handleGetMyAISettings)
	mux.HandleFunc("PUT /api/v1/ai/settings", a.handleSaveMyAISettings)
	mux.HandleFunc("PATCH /api/v1/ai/settings", a.handlePatchMyAISettings)
	mux.HandleFunc("GET /api/v1/ai/tenant-settings", a.handleGetTenantAISettings)
	mux.HandleFunc("PUT /api/v1/ai/tenant-settings", a.handleSaveTenantAISettings)
	mux.HandleFunc("PATCH /api/v1/ai/tenant-settings", a.handlePatchTenantAISettings)
	mux.HandleFunc("GET /api/v1/tenants/{id}/ai-settings", a.handleGetTenantAISettings)
	mux.HandleFunc("PUT /api/v1/tenants/{id}/ai-settings", a.handleSaveTenantAISettings)
	mux.HandleFunc("PATCH /api/v1/tenants/{id}/ai-settings", a.handlePatchTenantAISettings)
	mux.HandleFunc("GET /api/v1/ai/models", a.handleListAIModels)
}

func (a *App) handleGetMyAISettings(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	settings, err := a.loadUserAISettings(r.Context(), auth.UserID)
	if err != nil {
		a.logger.Error("load user ai settings failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "ai_settings_failed", "Could not load AI settings", r)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *App) handleSaveMyAISettings(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	var req aiProviderSettingsRequest
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	settings, fields, err := a.saveUserAISettings(r.Context(), auth, req)
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}
	if err != nil {
		a.logger.Error("save user ai settings failed", "error", err)
		writeErrorJSON(w, http.StatusBadGateway, "ai_provider_check_failed", "Could not validate AI provider", r)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *App) handlePatchMyAISettings(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	if req.Enabled == nil {
		writeValidationError(w, map[string]string{"enabled": "Enabled is required"}, r)
		return
	}
	_, err := a.db.ExecContext(r.Context(), `UPDATE user_ai_provider_settings SET enabled=$2, updated_at=now() WHERE user_id=$1`, auth.UserID, *req.Enabled)
	if err != nil {
		a.logger.Error("patch user ai settings failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "ai_settings_failed", "Could not update AI settings", r)
		return
	}
	settings, _ := a.loadUserAISettings(r.Context(), auth.UserID)
	writeJSON(w, http.StatusOK, settings)
}

func (a *App) handleGetTenantAISettings(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "tenants:write") {
		return
	}
	tenantID := a.aiSettingsTenantID(r)
	if tenantID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "tenant_required", "Tenant is required", r)
		return
	}
	settings, err := a.loadTenantAISettings(r.Context(), tenantID)
	if err != nil {
		a.logger.Error("load tenant ai settings failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "ai_settings_failed", "Could not load tenant AI settings", r)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *App) handleSaveTenantAISettings(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "tenants:write") {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	auth := AuthFromContext(r.Context())
	tenantID := a.aiSettingsTenantID(r)
	if tenantID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "tenant_required", "Tenant is required", r)
		return
	}
	var req aiProviderSettingsRequest
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	settings, fields, err := a.saveTenantAISettings(r.Context(), auth, tenantID, req)
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}
	if err != nil {
		a.logger.Error("save tenant ai settings failed", "error", err)
		writeErrorJSON(w, http.StatusBadGateway, "ai_provider_check_failed", "Could not validate AI provider", r)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *App) handlePatchTenantAISettings(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "tenants:write") {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	auth := AuthFromContext(r.Context())
	tenantID := a.aiSettingsTenantID(r)
	if tenantID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "tenant_required", "Tenant is required", r)
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	if req.Enabled == nil {
		writeValidationError(w, map[string]string{"enabled": "Enabled is required"}, r)
		return
	}
	_, err := a.db.ExecContext(r.Context(), `UPDATE tenant_ai_provider_settings SET enabled=$3, updated_by=$2, updated_at=now() WHERE tenant_id=$1`, tenantID, auth.UserID, *req.Enabled)
	if err != nil {
		a.logger.Error("patch tenant ai settings failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "ai_settings_failed", "Could not update tenant AI settings", r)
		return
	}
	settings, _ := a.loadTenantAISettings(r.Context(), tenantID)
	writeJSON(w, http.StatusOK, settings)
}

func (a *App) handleListAIModels(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	tenantID := ""
	if auth.EffectiveTenantID != nil {
		tenantID = *auth.EffectiveTenantID
	}
	provider, err := a.resolveAIProvider(r.Context(), auth, tenantID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"scope": "environment", "defaultModel": "MORFOSCHOOLS", "models": []aiModelInfo{{ID: "MORFOSCHOOLS"}}})
		return
	}
	models := provider.Models
	if len(models) == 0 && provider.DefaultModel != "" {
		models = []aiModelInfo{{ID: provider.DefaultModel}}
	}
	writeJSON(w, http.StatusOK, map[string]any{"scope": provider.Scope, "defaultModel": provider.DefaultModel, "models": models})
}

func (a *App) aiSettingsTenantID(r *http.Request) string {
	if id := strings.TrimSpace(r.PathValue("id")); id != "" {
		return id
	}
	if auth := AuthFromContext(r.Context()); auth != nil && auth.EffectiveTenantID != nil {
		return *auth.EffectiveTenantID
	}
	return ""
}

func (a *App) loadUserAISettings(ctx context.Context, userID string) (aiProviderSettingsResponse, error) {
	var baseURL, encryptedKey string
	var defaultModel sql.NullString
	var availableRaw, chatbotRaw []byte
	var enabled bool
	var updatedAt time.Time
	err := a.db.QueryRowContext(ctx, `SELECT base_url, encrypted_api_key, default_model, available_models, chatbot_models, enabled, updated_at FROM user_ai_provider_settings WHERE user_id=$1`, userID).Scan(&baseURL, &encryptedKey, &defaultModel, &availableRaw, &chatbotRaw, &enabled, &updatedAt)
	if err == sql.ErrNoRows {
		return aiProviderSettingsResponse{Scope: "user", Enabled: false, AvailableModels: []aiModelInfo{}, ChatbotModels: []aiModelInfo{}}, nil
	}
	if err != nil {
		return aiProviderSettingsResponse{}, err
	}
	return aiProviderSettingsResponse{Scope: "user", BaseURL: baseURL, HasAPIKey: encryptedKey != "", DefaultModel: defaultModel.String, AvailableModels: parseAIModels(availableRaw), ChatbotModels: parseAIModels(chatbotRaw), Enabled: enabled, UpdatedAt: updatedAt.Format(time.RFC3339)}, nil
}

func (a *App) loadTenantAISettings(ctx context.Context, tenantID string) (aiProviderSettingsResponse, error) {
	var baseURL, encryptedKey string
	var defaultModel sql.NullString
	var availableRaw, chatbotRaw []byte
	var roles pqStringArray
	var enabled bool
	var updatedAt time.Time
	err := a.db.QueryRowContext(ctx, `SELECT base_url, encrypted_api_key, default_model, available_models, chatbot_models, allowed_roles, enabled, updated_at FROM tenant_ai_provider_settings WHERE tenant_id=$1`, tenantID).Scan(&baseURL, &encryptedKey, &defaultModel, &availableRaw, &chatbotRaw, &roles, &enabled, &updatedAt)
	if err == sql.ErrNoRows {
		return aiProviderSettingsResponse{Scope: "tenant", Enabled: false, AvailableModels: []aiModelInfo{}, ChatbotModels: []aiModelInfo{}, AllowedRoles: []string{}}, nil
	}
	if err != nil {
		return aiProviderSettingsResponse{}, err
	}
	return aiProviderSettingsResponse{Scope: "tenant", BaseURL: baseURL, HasAPIKey: encryptedKey != "", DefaultModel: defaultModel.String, AvailableModels: parseAIModels(availableRaw), ChatbotModels: parseAIModels(chatbotRaw), AllowedRoles: []string(roles), Enabled: enabled, UpdatedAt: updatedAt.Format(time.RFC3339)}, nil
}

func (a *App) saveUserAISettings(ctx context.Context, auth *AuthContext, req aiProviderSettingsRequest) (aiProviderSettingsResponse, map[string]string, error) {
	fields := validateAIProviderRequest(req, false)
	if len(fields) > 0 {
		return aiProviderSettingsResponse{}, fields, nil
	}
	apiKey, fields, err := a.resolveSubmittedAPIKey(ctx, "user", auth.UserID, req.APIKey)
	if len(fields) > 0 || err != nil {
		return aiProviderSettingsResponse{}, fields, err
	}
	models, fields, err := fetchAIModels(ctx, req.BaseURL, apiKey)
	if len(fields) > 0 || err != nil {
		return aiProviderSettingsResponse{}, fields, err
	}
	defaultModel := chooseDefaultModel(req.DefaultModel, models)
	chatbotModels := filterModelIDs(req.ChatbotModels, models)
	encrypted, err := a.encryptAIKey(apiKey)
	if err != nil {
		return aiProviderSettingsResponse{}, nil, err
	}
	tenantID := ""
	if auth.EffectiveTenantID != nil {
		tenantID = *auth.EffectiveTenantID
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	modelsJSON, _ := json.Marshal(models)
	chatbotJSON, _ := json.Marshal(chatbotModels)
	_, err = a.db.ExecContext(ctx, `INSERT INTO user_ai_provider_settings (user_id, tenant_id, base_url, encrypted_api_key, default_model, available_models, chatbot_models, enabled) VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8) ON CONFLICT (user_id) DO UPDATE SET tenant_id=EXCLUDED.tenant_id, base_url=EXCLUDED.base_url, encrypted_api_key=EXCLUDED.encrypted_api_key, default_model=EXCLUDED.default_model, available_models=EXCLUDED.available_models, chatbot_models=EXCLUDED.chatbot_models, enabled=EXCLUDED.enabled, updated_at=now()`, auth.UserID, tenantID, strings.TrimRight(req.BaseURL, "/"), encrypted, defaultModel, modelsJSON, chatbotJSON, enabled)
	if err != nil {
		return aiProviderSettingsResponse{}, nil, err
	}
	settings, err := a.loadUserAISettings(ctx, auth.UserID)
	return settings, nil, err
}

func (a *App) saveTenantAISettings(ctx context.Context, auth *AuthContext, tenantID string, req aiProviderSettingsRequest) (aiProviderSettingsResponse, map[string]string, error) {
	fields := validateAIProviderRequest(req, true)
	if len(fields) > 0 {
		return aiProviderSettingsResponse{}, fields, nil
	}
	apiKey, fields, err := a.resolveSubmittedAPIKey(ctx, "tenant", tenantID, req.APIKey)
	if len(fields) > 0 || err != nil {
		return aiProviderSettingsResponse{}, fields, err
	}
	models, fields, err := fetchAIModels(ctx, req.BaseURL, apiKey)
	if len(fields) > 0 || err != nil {
		return aiProviderSettingsResponse{}, fields, err
	}
	defaultModel := chooseDefaultModel(req.DefaultModel, models)
	chatbotModels := filterModelIDs(req.ChatbotModels, models)
	encrypted, err := a.encryptAIKey(apiKey)
	if err != nil {
		return aiProviderSettingsResponse{}, nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	modelsJSON, _ := json.Marshal(models)
	chatbotJSON, _ := json.Marshal(chatbotModels)
	roles := sanitizeRoleSlugs(req.AllowedRoles)
	_, err = a.db.ExecContext(ctx, `INSERT INTO tenant_ai_provider_settings (tenant_id, base_url, encrypted_api_key, default_model, available_models, chatbot_models, allowed_roles, enabled, created_by, updated_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9) ON CONFLICT (tenant_id) DO UPDATE SET base_url=EXCLUDED.base_url, encrypted_api_key=EXCLUDED.encrypted_api_key, default_model=EXCLUDED.default_model, available_models=EXCLUDED.available_models, chatbot_models=EXCLUDED.chatbot_models, allowed_roles=EXCLUDED.allowed_roles, enabled=EXCLUDED.enabled, updated_by=EXCLUDED.updated_by, updated_at=now()`, tenantID, strings.TrimRight(req.BaseURL, "/"), encrypted, defaultModel, modelsJSON, chatbotJSON, pqStringArrayValue(roles), enabled, auth.UserID)
	if err != nil {
		return aiProviderSettingsResponse{}, nil, err
	}
	settings, err := a.loadTenantAISettings(ctx, tenantID)
	return settings, nil, err
}

func validateAIProviderRequest(req aiProviderSettingsRequest, tenant bool) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(req.BaseURL) == "" {
		fields["baseUrl"] = "AI Base URL is required"
	}
	if strings.TrimSpace(req.APIKey) == "" {
		fields["apiKey"] = "API Key is required"
	}
	if strings.TrimSpace(req.BaseURL) != "" && !(strings.HasPrefix(req.BaseURL, "http://") || strings.HasPrefix(req.BaseURL, "https://")) {
		fields["baseUrl"] = "Base URL must start with http:// or https://"
	}
	return fields
}

func (a *App) resolveSubmittedAPIKey(ctx context.Context, scope, id, submitted string) (string, map[string]string, error) {
	if submitted != maskedAIKey {
		return strings.TrimSpace(submitted), nil, nil
	}
	var encrypted string
	var err error
	if scope == "user" {
		err = a.db.QueryRowContext(ctx, `SELECT encrypted_api_key FROM user_ai_provider_settings WHERE user_id=$1`, id).Scan(&encrypted)
	} else {
		err = a.db.QueryRowContext(ctx, `SELECT encrypted_api_key FROM tenant_ai_provider_settings WHERE tenant_id=$1`, id).Scan(&encrypted)
	}
	if err == sql.ErrNoRows {
		return "", map[string]string{"apiKey": "API Key is required"}, nil
	}
	if err != nil {
		return "", nil, err
	}
	apiKey, err := a.decryptAIKey(encrypted)
	return apiKey, nil, err
}

func (a *App) resolveAIProvider(ctx context.Context, auth *AuthContext, tenantID string) (resolvedAIProvider, error) {
	if auth != nil && auth.UserID != "" {
		if provider, ok := a.resolveUserAIProvider(ctx, auth.UserID); ok {
			return provider, nil
		}
	}
	if tenantID != "" && auth != nil {
		if provider, ok := a.resolveTenantAIProvider(ctx, tenantID, auth.Roles); ok {
			return provider, nil
		}
	}
	baseURL := strings.TrimRight(os.Getenv("AI_BASE_URL"), "/")
	apiKey := os.Getenv("AI_API_KEY")
	model := os.Getenv("AI_MODEL")
	if model == "" {
		model = "MORFOSCHOOLS"
	}
	if baseURL == "" || apiKey == "" {
		return resolvedAIProvider{}, fmt.Errorf("AI_BASE_URL/AI_API_KEY is not configured")
	}
	return resolvedAIProvider{Scope: "environment", BaseURL: baseURL, APIKey: apiKey, DefaultModel: model, Models: []aiModelInfo{{ID: model}}}, nil
}

func (a *App) resolveUserAIProvider(ctx context.Context, userID string) (resolvedAIProvider, bool) {
	var baseURL, encrypted, defaultModel string
	var chatbotRaw, availableRaw []byte
	err := a.db.QueryRowContext(ctx, `SELECT base_url, encrypted_api_key, COALESCE(default_model,''), chatbot_models, available_models FROM user_ai_provider_settings WHERE user_id=$1 AND enabled=true`, userID).Scan(&baseURL, &encrypted, &defaultModel, &chatbotRaw, &availableRaw)
	if err != nil {
		return resolvedAIProvider{}, false
	}
	apiKey, err := a.decryptAIKey(encrypted)
	if err != nil {
		return resolvedAIProvider{}, false
	}
	models := parseAIModels(chatbotRaw)
	if len(models) == 0 {
		models = parseAIModels(availableRaw)
	}
	return resolvedAIProvider{Scope: "user", BaseURL: baseURL, APIKey: apiKey, DefaultModel: defaultModel, Models: models}, true
}

func (a *App) resolveTenantAIProvider(ctx context.Context, tenantID string, userRoles []string) (resolvedAIProvider, bool) {
	var baseURL, encrypted, defaultModel string
	var allowed pqStringArray
	var chatbotRaw, availableRaw []byte
	err := a.db.QueryRowContext(ctx, `SELECT base_url, encrypted_api_key, COALESCE(default_model,''), allowed_roles, chatbot_models, available_models FROM tenant_ai_provider_settings WHERE tenant_id=$1 AND enabled=true`, tenantID).Scan(&baseURL, &encrypted, &defaultModel, &allowed, &chatbotRaw, &availableRaw)
	if err != nil {
		return resolvedAIProvider{}, false
	}
	if len(allowed) > 0 && !rolesIntersect([]string(allowed), userRoles) {
		return resolvedAIProvider{}, false
	}
	apiKey, err := a.decryptAIKey(encrypted)
	if err != nil {
		return resolvedAIProvider{}, false
	}
	models := parseAIModels(chatbotRaw)
	if len(models) == 0 {
		models = parseAIModels(availableRaw)
	}
	return resolvedAIProvider{Scope: "tenant", BaseURL: baseURL, APIKey: apiKey, DefaultModel: defaultModel, Models: models}, true
}

func fetchAIModels(ctx context.Context, baseURL, apiKey string) ([]aiModelInfo, map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, map[string]string{"baseUrl": "Could not connect to provider /models"}, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, map[string]string{"apiKey": fmt.Sprintf("Provider rejected credentials or URL (status %d)", resp.StatusCode)}, nil
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, map[string]string{"baseUrl": "Provider /models did not return valid JSON"}, nil
	}
	models := []aiModelInfo{}
	for _, m := range parsed.Data {
		if strings.TrimSpace(m.ID) != "" {
			models = append(models, aiModelInfo{ID: strings.TrimSpace(m.ID)})
		}
	}
	if len(models) == 0 {
		return nil, map[string]string{"baseUrl": "Provider returned no models"}, nil
	}
	return models, nil, nil
}

func (a *App) callLLMWithProvider(ctx context.Context, provider resolvedAIProvider, messages []llmMessage) (*llmResponse, error) {
	model := provider.DefaultModel
	if model == "" && len(provider.Models) > 0 {
		model = provider.Models[0].ID
	}
	if model == "" {
		model = "MORFOSCHOOLS"
	}
	body := map[string]any{"model": model, "messages": messages, "temperature": 0.4, "max_tokens": 1200}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(provider.BaseURL, "/")+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	return doLLMRequest(httpReq)
}

func doLLMRequest(httpReq *http.Request) (*llmResponse, error) {
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	jsonBody2 := respBody
	if idx := bytes.Index(jsonBody2, []byte("data: [DONE]")); idx > 0 {
		jsonBody2 = bytes.TrimSpace(jsonBody2[:idx])
	}
	if idx := bytes.Index(jsonBody2, []byte("\ndata: ")); idx > 0 {
		jsonBody2 = bytes.TrimSpace(jsonBody2[:idx])
	}
	var direct llmResponse
	if json.Unmarshal(jsonBody2, &direct) == nil && len(direct.Choices) > 0 {
		return &direct, nil
	}
	if bytes.Contains(respBody, []byte("data: ")) {
		var content strings.Builder
		var usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}
		for _, line := range bytes.Split(respBody, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data: ")) || bytes.Equal(line, []byte("data: [DONE]")) {
				continue
			}
			var parsed struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal(bytes.TrimPrefix(line, []byte("data: ")), &parsed) != nil {
				continue
			}
			if len(parsed.Choices) > 0 {
				content.WriteString(parsed.Choices[0].Delta.Content)
				content.WriteString(parsed.Choices[0].Message.Content)
			}
			if parsed.Usage.TotalTokens > 0 {
				usage = parsed.Usage
			}
		}
		return &llmResponse{Choices: []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{{Message: struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "assistant", Content: content.String()}}}, Usage: usage}, nil
	}
	return nil, fmt.Errorf("failed to parse LLM response: %s", string(respBody[:min(len(respBody), 200)]))
}

func parseAIModels(raw []byte) []aiModelInfo {
	var models []aiModelInfo
	_ = json.Unmarshal(raw, &models)
	if models == nil {
		return []aiModelInfo{}
	}
	return models
}

func chooseDefaultModel(selected string, models []aiModelInfo) string {
	selected = strings.TrimSpace(selected)
	if selected != "" {
		for _, m := range models {
			if m.ID == selected {
				return selected
			}
		}
	}
	if len(models) > 0 {
		return models[0].ID
	}
	return selected
}

func filterModelIDs(ids []string, models []aiModelInfo) []aiModelInfo {
	allowed := map[string]bool{}
	for _, m := range models {
		allowed[m.ID] = true
	}
	out := []aiModelInfo{}
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && allowed[id] && !seen[id] {
			out = append(out, aiModelInfo{ID: id})
			seen[id] = true
		}
	}
	return out
}

func sanitizeRoleSlugs(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, role := range in {
		role = strings.TrimSpace(role)
		if role != "" && !seen[role] {
			out = append(out, role)
			seen[role] = true
		}
	}
	return out
}

func rolesIntersect(allowed, current []string) bool {
	for _, role := range current {
		if slices.Contains(allowed, role) {
			return true
		}
	}
	return false
}

func (a *App) encryptAIKey(plain string) (string, error) {
	key := a.aiEncryptionKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func (a *App) decryptAIKey(encoded string) (string, error) {
	key := a.aiEncryptionKey()
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("invalid encrypted key")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (a *App) aiEncryptionKey() [32]byte {
	seed := os.Getenv("AI_SETTINGS_ENCRYPTION_KEY")
	if seed == "" {
		seed = os.Getenv("AI_API_KEY")
	}
	if seed == "" {
		seed = "morfoschools-development-ai-settings-key"
	}
	return sha256.Sum256([]byte(seed))
}

// lightweight pq text[] support without importing lib/pq.
type pqStringArray []string

func (a *pqStringArray) Scan(src any) error {
	if src == nil {
		*a = []string{}
		return nil
	}
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("unsupported array source")
	}
	s = strings.Trim(s, "{}")
	if s == "" {
		*a = []string{}
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.Trim(strings.TrimSpace(p), `"`))
	}
	*a = out
	return nil
}

type pqStringArrayValue []string

func (a pqStringArrayValue) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	escaped := make([]string, len(a))
	for i, s := range a {
		escaped[i] = `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return "{" + strings.Join(escaped, ",") + "}", nil
}
