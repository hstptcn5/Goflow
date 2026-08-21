package nodes

import (
	"encoding/json"
	"testing"
)

func TestParseCURLCommandMigratesBearerSecretOutOfWorkflowParams(t *testing.T) {
	result, err := ParseCURLCommand(`curl -X POST 'https://api.example.com/items?q=1' -H 'Authorization: Bearer super-secret' -H 'Accept: application/json' -d '{"name":"demo"}'`)
	if err != nil {
		t.Fatal(err)
	}
	if result.CredentialSecret != "super-secret" || result.CredentialHint != "Authorization" {
		t.Fatalf("credential migration = %#v", result)
	}
	if result.Params["auth_mode"] != "bearer" || result.Params["method"] != "POST" {
		t.Fatalf("params = %#v", result.Params)
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(result.Params["headers"].(string)), &headers); err != nil {
		t.Fatal(err)
	}
	if headers["Accept"] != "application/json" {
		t.Fatalf("headers = %#v", headers)
	}
	if _, leaked := headers["Authorization"]; leaked {
		t.Fatalf("Authorization secret leaked into workflow headers: %#v", headers)
	}
}

func TestParseCURLCommandInfersPostAndRawBody(t *testing.T) {
	result, err := ParseCURLCommand(`curl https://example.com/hook --data-raw 'hello world'`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Params["method"] != "POST" || result.Params["body_mode"] != "raw" || result.Params["body"] != "hello world" {
		t.Fatalf("params = %#v", result.Params)
	}
}

func TestParseCURLCommandBasicAuthIsReturnedAsCredentialSecret(t *testing.T) {
	result, err := ParseCURLCommand(`curl -u 'alice:password' https://example.com/private`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Params["auth_mode"] != "basic" || result.CredentialSecret != "alice:password" {
		t.Fatalf("result = %#v", result)
	}
}

func TestParseCURLCommandRejectsUnsupportedOrMultipleCredentials(t *testing.T) {
	if _, err := ParseCURLCommand(`curl --cert client.pem https://example.com`); err == nil {
		t.Fatal("unsupported cURL option was accepted")
	}
	if _, err := ParseCURLCommand(`curl -u a:b -H 'X-API-Key: secret' https://example.com`); err == nil {
		t.Fatal("multiple credentials were accepted")
	}
}
