package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"goflow/internal/apperror"
	"goflow/internal/jsoncontract"
	"goflow/internal/nodes"
	"goflow/internal/pack"
	"goflow/internal/packsetup"
	"goflow/internal/sourceprobe"
	"goflow/internal/storage"
)

const telegramTestResponseLimit int64 = 64 << 10

type applianceValidationResult struct {
	Category string                 `json:"category,omitempty"`
	Message  string                 `json:"message,omitempty"`
	Status   string                 `json:"status"`
	Summary  map[string]interface{} `json:"summary,omitempty"`
}

func applianceTestSourceHandler(appliance *ApplianceContext, wfStore *storage.WorkflowStore, limiter *fixedWindowRateLimiter, slots chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(rateLimitKey(r, appliance.PackID+":source-test")) {
			renderJSON(w, http.StatusTooManyRequests, applianceValidationResult{Status: "INVALID", Category: "rate_limited", Message: "Source testing is temporarily rate limited. Try again shortly."})
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			renderJSON(w, http.StatusTooManyRequests, applianceValidationResult{Status: "INVALID", Category: "test_already_running", Message: "A source test is already running."})
			return
		}
		var req struct {
			Key string `json:"key"`
		}
		if err := decodeApplianceJSON(w, r, &req); err != nil {
			return
		}
		result, err := applianceValidateSource(r.Context(), appliance, wfStore, strings.TrimSpace(req.Key))
		if err != nil {
			writeApplianceValidationError(w, http.StatusBadGateway, err)
			return
		}
		renderJSON(w, http.StatusOK, applianceValidationResult{
			Status: "VALID",
			Summary: map[string]interface{}{
				"report_date":  result.Summary.ReportDate,
				"valid_fields": result.Summary.RequiredFields,
			},
		})
	}
}

func applianceValidateSource(ctx context.Context, appliance *ApplianceContext, wfStore *storage.WorkflowStore, key string) (*sourceprobe.Result, error) {
	field, ok := applianceConfigField(appliance, key)
	if !ok || field.TestKind != "http_json_contract" {
		return nil, apperror.New("source_invalid_url", "This configuration field does not declare a source test.")
	}
	loaded, err := packsetup.LoadConfig(appliance.DataDir, applianceManifest(appliance))
	if err != nil {
		return nil, apperror.New("source_invalid_url", "Save a valid source URL before testing it.")
	}
	rawURL, ok := loaded.Config.Values[key].(string)
	if !ok {
		return nil, apperror.New("source_invalid_url", "Save a valid source URL before testing it.")
	}
	contract, err := applianceHTTPResponseContract(appliance, wfStore, key)
	if err != nil {
		return nil, apperror.New("internal_error", "The source contract is not available. Reinstall the verified pack bundle.")
	}
	return sourceprobe.Check(ctx, appliance.ConnectionTestClient, rawURL, contract, sourceprobe.DefaultMaxResponseBytes)
}

func applianceHTTPResponseContract(appliance *ApplianceContext, wfStore *storage.WorkflowStore, key string) (jsoncontract.Contract, error) {
	if wfStore == nil {
		return jsoncontract.Contract{}, fmt.Errorf("workflow store is unavailable")
	}
	wf, err := wfStore.GetByID(appliance.WorkflowID)
	if err != nil {
		return jsoncontract.Contract{}, err
	}
	var workflowNodes []nodes.Node
	if err := json.Unmarshal([]byte(wf.NodesJSON), &workflowNodes); err != nil {
		return jsoncontract.Contract{}, err
	}
	for _, binding := range appliance.Bindings {
		if binding.Source != "config."+key || binding.Target.Param != "url" {
			continue
		}
		for _, node := range workflowNodes {
			if node.ID != binding.Target.NodeID || node.Type != nodes.TypeHTTPRequest {
				continue
			}
			return jsoncontract.Parse(node.Params["response_contract"])
		}
	}
	return jsoncontract.Contract{}, fmt.Errorf("source node is unavailable")
}

func applianceValidateCredentialDestination(ctx context.Context, appliance *ApplianceContext, credStore *storage.CredentialStore, key string) error {
	if credStore == nil {
		return apperror.New("internal_error", "Credential storage is not available.")
	}
	requirement, ok := credentialRequirement(appliance, key)
	if !ok {
		return apperror.New("internal_error", "The credential slot is not declared by this pack.")
	}
	if requirement.TestKind == "" {
		return nil
	}
	if requirement.TestKind != "telegram_get_me" {
		return apperror.New("internal_error", "This credential test is not supported by the appliance.")
	}
	loaded, err := packsetup.LoadCredentialBindings(appliance.DataDir, applianceManifest(appliance), applianceCredentialResolver(credStore))
	if err != nil {
		return apperror.New("telegram_unauthorized", "Save a Telegram bot token before testing it.")
	}
	slot, ok := loaded.Credentials.Slots[key]
	if !ok {
		return apperror.New("telegram_unauthorized", "Save a Telegram bot token before testing it.")
	}
	secret, err := credStore.GetDecryptedData(slot.CredentialID)
	if err != nil {
		return apperror.New("telegram_unauthorized", "The Telegram bot token is not available.")
	}
	chatID, err := applianceTelegramChatID(appliance, key)
	if err != nil {
		return apperror.New("telegram_chat_inaccessible", "Save a Telegram chat ID before testing the destination.")
	}
	return applianceTelegramDestinationCheck(ctx, appliance, secret, chatID)
}

func applianceTelegramChatID(appliance *ApplianceContext, credentialKey string) (string, error) {
	credentialNode := ""
	for _, binding := range appliance.Bindings {
		if binding.Source == "credential."+credentialKey {
			credentialNode = binding.Target.NodeID
			break
		}
	}
	if credentialNode == "" {
		return "", fmt.Errorf("credential binding is unavailable")
	}
	config, err := packsetup.LoadConfig(appliance.DataDir, applianceManifest(appliance))
	if err != nil {
		return "", err
	}
	for _, binding := range appliance.Bindings {
		if binding.Target.NodeID != credentialNode || binding.Target.Param != "chat_id" || !strings.HasPrefix(binding.Source, "config.") {
			continue
		}
		value, ok := config.Config.Values[strings.TrimPrefix(binding.Source, "config.")].(string)
		if ok && strings.TrimSpace(value) != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("chat binding is unavailable")
}

func applianceTelegramDestinationCheck(ctx context.Context, appliance *ApplianceContext, token, chatID string) error {
	base := strings.TrimRight(appliance.TelegramAPIBaseURL, "/")
	if base == "" {
		base = "https://api.telegram.org"
	}
	parsed, err := url.Parse(base)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return apperror.New("internal_error", "The Telegram connection test is not available.")
	}
	client := telegramConnectionTestClient(appliance.ConnectionTestClient)
	if err := telegramTestMethod(ctx, client, fmt.Sprintf("%s/bot%s/getMe", base, token), "telegram_unauthorized", "Invalid bot token. Check the token from BotFather and try again."); err != nil {
		return err
	}
	getChatURL := fmt.Sprintf("%s/bot%s/getChat?chat_id=%s", base, token, url.QueryEscape(chatID))
	return telegramTestMethod(ctx, client, getChatURL, "telegram_chat_inaccessible", "Bot cannot access this chat. Send /start to the bot, add it to the destination, and check the chat ID.")
}

func telegramConnectionTestClient(base *http.Client) *http.Client {
	if base == nil {
		return &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	client := *base
	if client.Timeout <= 0 || client.Timeout > 10*time.Second {
		client.Timeout = 10 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client
}

func telegramTestMethod(ctx context.Context, client *http.Client, requestURL, category, message string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return apperror.New("internal_error", "The Telegram connection test could not be started.")
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return apperror.New("telegram_unreachable", "Telegram did not respond within 10 seconds.")
		}
		return apperror.New("telegram_unreachable", "Goflow could not connect to Telegram. Check the network and try again.")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, telegramTestResponseLimit+1))
	if err != nil || int64(len(data)) > telegramTestResponseLimit {
		return apperror.New("telegram_unreachable", "Goflow could not read Telegram's response. Try again.")
	}
	var decoded struct {
		OK bool `json:"ok"`
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return apperror.New("telegram_unauthorized", "Invalid bot token. Check the token from BotFather and try again.")
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		return apperror.New("telegram_unreachable", "Telegram is temporarily unavailable. Try again shortly.")
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(data, &decoded) != nil || !decoded.OK {
		return apperror.New(category, message)
	}
	return nil
}

func applianceValidateCompletion(ctx context.Context, appliance *ApplianceContext, wfStore *storage.WorkflowStore, credStore *storage.CredentialStore) error {
	for _, field := range appliance.ConfigSchema {
		if field.Required && field.TestKind != "" {
			if _, err := applianceValidateSource(ctx, appliance, wfStore, field.Key); err != nil {
				return err
			}
		}
	}
	for _, requirement := range appliance.CredentialRequirements {
		if requirement.Required && requirement.TestKind != "" {
			if err := applianceValidateCredentialDestination(ctx, appliance, credStore, requirement.Key); err != nil {
				return err
			}
		}
	}
	return nil
}

func applianceConfigField(appliance *ApplianceContext, key string) (pack.ConfigField, bool) {
	for _, field := range appliance.ConfigSchema {
		if field.Key == key {
			return field, true
		}
	}
	return pack.ConfigField{}, false
}

func writeApplianceValidationError(w http.ResponseWriter, status int, err error) {
	category, message, ok := apperror.Details(err)
	if !ok {
		category = "internal_error"
		message = "The request could not be completed. Refresh and try again."
		status = http.StatusInternalServerError
	}
	if errors.Is(err, context.Canceled) {
		status = http.StatusRequestTimeout
	}
	if publicCategory, publicMessage, supported := apperror.Public(category); supported {
		category = publicCategory
		message = publicMessage
	} else {
		category = apperror.CategoryInternal
		message = "The request could not be completed. Refresh and try again."
		status = http.StatusInternalServerError
	}
	renderJSON(w, status, applianceValidationResult{Status: "INVALID", Category: category, Message: message})
}

func appliancePublicExecutionError(status, raw string) (string, string) {
	if strings.TrimSpace(raw) == "" {
		return "", ""
	}
	if category, message, ok := apperror.DetailsText(raw); ok {
		_ = message
		if publicCategory, publicMessage, supported := apperror.Public(category); supported {
			return publicCategory, publicMessage
		}
	}
	if strings.EqualFold(status, "CANCELLED") || strings.EqualFold(status, "INTERRUPTED") {
		category, message, _ := apperror.Public(strings.ToLower(status))
		return category, message
	}
	return apperror.CategoryInternal, "The workflow could not complete. Open redacted diagnostics when reporting this error."
}
