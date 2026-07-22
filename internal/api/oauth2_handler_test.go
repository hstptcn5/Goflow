package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"goflow/internal/crypto"
	"goflow/internal/storage"
)

func TestOAuth2FlowOffline(t *testing.T) {
	dbPath := "./test_api_oauth2.db"
	// Clean up any stale DB file
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	defer func() {
		_ = os.Remove(dbPath)
		_ = os.Remove(dbPath + "-wal")
		_ = os.Remove(dbPath + "-shm")
	}()

	db, err := storage.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize test DB: %v", err)
	}
	defer db.Close()

	cm := crypto.NewCryptoManager("0123456789abcdef0123456789abcdef")
	credStore := storage.NewCredentialStore(db, cm)
	handler := NewOAuth2Handler(credStore)

	// Test 1: Authorize with missing params
	req1 := httptest.NewRequest("GET", "/api/v1/oauth2/authorize", nil)
	rr1 := httptest.NewRecorder()
	handler.Authorize(rr1, req1)
	if rr1.Code != http.StatusBadRequest {
		t.Errorf("Expected status Bad Request (400), got: %d", rr1.Code)
	}

	// Test 2: Successful Authorize request creation and redirect
	query := url.Values{}
	query.Set("name", "Test Google OAuth2")
	query.Set("client_id", "my-client-id")
	query.Set("client_secret", "my-client-secret")
	query.Set("auth_url", "https://accounts.google.com/o/oauth2/auth")
	query.Set("token_url", "https://oauth2.googleapis.com/token")
	query.Set("scopes", "https://www.googleapis.com/auth/spreadsheets")

	req2 := httptest.NewRequest("GET", "/api/v1/oauth2/authorize?"+query.Encode(), nil)
	rr2 := httptest.NewRecorder()
	handler.Authorize(rr2, req2)

	if rr2.Code != http.StatusFound {
		t.Errorf("Expected status Found (302), got: %d", rr2.Code)
	}

	loc := rr2.Header().Get("Location")
	if !strings.Contains(loc, "accounts.google.com") {
		t.Errorf("Expected redirect URL to contain accounts.google.com, got: %s", loc)
	}

	// Verify temporary credential got created
	creds, err := credStore.ListAll()
	if err != nil || len(creds) != 1 {
		t.Fatalf("Expected 1 credential created, got: %d (error: %v)", len(creds), err)
	}

	cred := creds[0]
	if cred.Name != "Test Google OAuth2" || cred.Type != "oauth2" {
		t.Errorf("Expected credential properties mismatch, got Name: %s, Type: %s", cred.Name, cred.Type)
	}

	// Test 3: Callback fails with invalid/missing state or code
	req3 := httptest.NewRequest("GET", "/api/v1/oauth2/callback", nil)
	rr3 := httptest.NewRecorder()
	handler.Callback(rr3, req3)
	if rr3.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing code/state, got: %d", rr3.Code)
	}
}
