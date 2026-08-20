package nodes

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/oauth2/google"
)

type googleAuthMaterial struct {
	AccessToken        string
	ServiceAccountJSON string
}

func resolveGoogleAuth(ctx *ExecutionContext, node *Node) (googleAuthMaterial, error) {
	credentialID, _ := node.Params["credential_id"].(string)
	credentialID = strings.TrimSpace(credentialID)
	directSA, _ := node.Params["service_account_json"].(string)
	directSA = strings.TrimSpace(directSA)
	if credentialID == "" {
		if directSA == "" {
			return googleAuthMaterial{}, fmt.Errorf("Google credential or service_account_json is required")
		}
		return googleAuthMaterial{ServiceAccountJSON: directSA}, nil
	}
	if ctx == nil {
		return googleAuthMaterial{}, fmt.Errorf("Google credential context is unavailable")
	}
	var secret string
	var err error
	if ctx.RefreshCredential != nil {
		secret, err = ctx.RefreshCredential(credentialID)
		if err != nil {
			return googleAuthMaterial{}, fmt.Errorf("Google credential %q could not be refreshed: %w", credentialID, err)
		}
	} else {
		secret = ctx.Credentials[credentialID]
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return googleAuthMaterial{}, fmt.Errorf("Google credential %q is not available", credentialID)
	}
	if strings.HasPrefix(secret, "{") {
		return googleAuthMaterial{ServiceAccountJSON: secret}, nil
	}
	return googleAuthMaterial{AccessToken: secret}, nil
}

func googleAccessToken(ctx context.Context, material googleAuthMaterial, scopes ...string) (string, error) {
	return googleAccessTokenForSubject(ctx, material, "", scopes...)
}

func googleAccessTokenForSubject(ctx context.Context, material googleAuthMaterial, subject string, scopes ...string) (string, error) {
	if strings.TrimSpace(material.AccessToken) != "" {
		return strings.TrimSpace(material.AccessToken), nil
	}
	if strings.TrimSpace(material.ServiceAccountJSON) == "" {
		return "", fmt.Errorf("Google authentication material is empty")
	}
	jwtConfig, err := google.JWTConfigFromJSON([]byte(material.ServiceAccountJSON), scopes...)
	if err != nil {
		return "", fmt.Errorf("invalid service account JSON: %w", err)
	}
	if strings.TrimSpace(subject) != "" {
		jwtConfig.Subject = strings.TrimSpace(subject)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	token, err := jwtConfig.TokenSource(ctx).Token()
	if err != nil {
		return "", fmt.Errorf("failed to generate Google OAuth2 token: %w", err)
	}
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return "", fmt.Errorf("Google OAuth2 token source returned an empty access token")
	}
	return token.AccessToken, nil
}
