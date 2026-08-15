package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goflow/internal/crypto"
	"goflow/internal/storage"
)

func TestParseServerURL(t *testing.T) {
	tests := []struct {
		name, logs, want string
		wantNotFound     bool
	}{
		{name: "valid", logs: "2026/08/15 12:00:00 [INFO] Goflow Web Server running on http://127.0.0.1:43127\n", want: "http://127.0.0.1:43127"},
		{name: "not yet logged", logs: "starting\n", wantNotFound: true},
		{name: "localhost rejected", logs: "[INFO] Goflow Web Server running on http://localhost:43127\n"},
		{name: "non-loopback rejected", logs: "[INFO] Goflow Web Server running on http://0.0.0.0:43127\n"},
		{name: "remote rejected", logs: "[INFO] Goflow Web Server running on http://example.com:43127\n"},
		{name: "https rejected", logs: "[INFO] Goflow Web Server running on https://127.0.0.1:43127\n"},
		{name: "missing port rejected", logs: "[INFO] Goflow Web Server running on http://127.0.0.1\n"},
		{name: "zero port rejected", logs: "[INFO] Goflow Web Server running on http://127.0.0.1:0\n"},
		{name: "oversized port rejected", logs: "[INFO] Goflow Web Server running on http://127.0.0.1:65536\n"},
		{name: "path rejected", logs: "[INFO] Goflow Web Server running on http://127.0.0.1:43127/admin\n"},
		{name: "userinfo rejected", logs: "[INFO] Goflow Web Server running on http://user@127.0.0.1:43127\n"},
		{name: "multiple rejected", logs: "[INFO] Goflow Web Server running on http://127.0.0.1:43127\n[INFO] Goflow Web Server running on http://127.0.0.1:43128\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServerURL([]byte(tt.logs))
			if tt.wantNotFound {
				if !errors.Is(err, errServerURLNotFound) {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if tt.want != "" {
				if err != nil || got != tt.want {
					t.Fatalf("got=%q error=%v", got, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("malformed URL was accepted as %q", got)
			}
		})
	}
}

func TestParseServerURLRejectsOversizedLog(t *testing.T) {
	if _, err := parseServerURL([]byte(strings.Repeat("x", maxStartupLogBytes+1))); err == nil {
		t.Fatal("oversized startup log was accepted")
	}
}

func TestVerifyCredentialAtRestUsesProductionDecryptionPath(t *testing.T) {
	dataDir := t.TempDir()
	key := "community-rc-test-master-key"
	if err := os.WriteFile(filepath.Join(dataDir, "goflow.master.key"), []byte(key+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := storage.NewDB(filepath.Join(dataDir, "goflow.db"))
	if err != nil {
		t.Fatal(err)
	}
	credential, err := storage.NewCredentialStore(db, crypto.NewCryptoManager(key)).Create("test", "API_KEY", secretCanary)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCredentialAtRest(dataDir, credential.ID); err != nil {
		t.Fatal(err)
	}
	if err := verifyCredentialAtRest(dataDir, "wrong-id"); err == nil {
		t.Fatal("wrong credential identity was accepted")
	}
}
