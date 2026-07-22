package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"goflow/internal/storage"

	"golang.org/x/oauth2"
)

type OAuth2Handler struct {
	credStore *storage.CredentialStore
}

func NewOAuth2Handler(cs *storage.CredentialStore) *OAuth2Handler {
	return &OAuth2Handler{credStore: cs}
}

type OAuthConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
	Scopes       string `json:"scopes"`
}

type OAuthCredentialPayload struct {
	Config OAuthConfig   `json:"config"`
	Token  *oauth2.Token `json:"token,omitempty"`
}

func (h *OAuth2Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	clientID := r.URL.Query().Get("client_id")
	clientSecret := r.URL.Query().Get("client_secret")
	authURL := r.URL.Query().Get("auth_url")
	tokenURL := r.URL.Query().Get("token_url")
	scopes := r.URL.Query().Get("scopes")

	if name == "" || clientID == "" || clientSecret == "" || authURL == "" || tokenURL == "" {
		http.Error(w, "Missing required OAuth2 parameters", http.StatusBadRequest)
		return
	}

	// 1. Create temporary credential containing configuration
	config := OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      authURL,
		TokenURL:     tokenURL,
		Scopes:       scopes,
	}

	payload := OAuthCredentialPayload{Config: config}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cred, err := h.credStore.Create(name, "oauth2", string(payloadBytes))
	if err != nil {
		http.Error(w, "Failed to create credential: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Build redirect auth URL
	var redirectURL = "http://localhost:8089/api/v1/oauth2/callback"
	if port := r.Header.Get("X-Forwarded-Port"); port != "" {
		redirectURL = fmt.Sprintf("http://localhost:%s/api/v1/oauth2/callback", port)
	}

	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}

	if scopes != "" {
		// Split scopes by space or comma
		var scopesList []string
		if strings.Contains(scopes, ",") {
			scopesList = strings.Split(scopes, ",")
		} else {
			scopesList = strings.Split(scopes, " ")
		}
		for i, s := range scopesList {
			scopesList[i] = strings.TrimSpace(s)
		}
		conf.Scopes = scopesList
	}

	// Redirect user to authorize page, forcing offline access type to obtain refresh_token
	authPageURL := conf.AuthCodeURL(cred.ID, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	http.Redirect(w, r, authPageURL, http.StatusFound)
}

func (h *OAuth2Handler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state") // state is the Credential ID

	if code == "" || state == "" {
		http.Error(w, "Authorization code or state param missing", http.StatusBadRequest)
		return
	}

	// 1. Get decrypted credential config
	_, err := h.credStore.GetByID(state)
	if err != nil {
		http.Error(w, "Credential not found: "+err.Error(), http.StatusNotFound)
		return
	}

	decryptedRaw, err := h.credStore.GetDecryptedData(state)
	if err != nil {
		http.Error(w, "Failed to decrypt credential data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var payload OAuthCredentialPayload
	if err := json.Unmarshal([]byte(decryptedRaw), &payload); err != nil {
		http.Error(w, "Failed to parse credential config payload: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Exchange code for token
	var redirectURL = "http://localhost:8089/api/v1/oauth2/callback"
	conf := &oauth2.Config{
		ClientID:     payload.Config.ClientID,
		ClientSecret: payload.Config.ClientSecret,
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  payload.Config.AuthURL,
			TokenURL: payload.Config.TokenURL,
		},
	}

	goCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := conf.Exchange(goCtx, code)
	if err != nil {
		http.Error(w, "Token exchange failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Save full Token payload back to credential store
	payload.Token = token
	updatedPayloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = h.credStore.UpdateData(state, string(updatedPayloadBytes))
	if err != nil {
		http.Error(w, "Failed to update credential: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Render success HTML page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>OAuth2 Authorization Success</title>
			<style>
				body {
					font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
					background: #f0fdf4;
					color: #166534;
					display: flex;
					align-items: center;
					justify-content: center;
					height: 100vh;
					margin: 0;
				}
				.card {
					background: white;
					padding: 40px;
					border-radius: 16px;
					box-shadow: 0 10px 25px rgba(22, 101, 52, 0.05);
					text-align: center;
					border: 2px solid #bbf7d0;
					max-width: 450px;
				}
				h1 { margin-top: 0; font-size: 1.8rem; }
				p { font-size: 1rem; color: #4b5563; }
				.icon { font-size: 3rem; margin-bottom: 20px; }
			</style>
		</head>
		<body>
			<div class="card">
				<div class="icon">🎉</div>
				<h1>Xác thực thành công!</h1>
				<p>Tài khoản OAuth2 đã được liên kết và lưu trữ bảo mật trong Goflow Vault.</p>
				<p>Bạn có thể đóng tab này để quay lại canvas.</p>
			</div>
		</body>
		</html>
	`))
}
