package nodes

import (
	"reflect"
	"testing"
)

func TestBuiltinRegistryDeclaresAIExtractCredentialCompatibility(t *testing.T) {
	registry := NewBuiltinRegistry()
	executor, ok := registry.Get(TypeAIExtract)
	if !ok {
		t.Fatal("AI Extract executor is not registered")
	}

	definition := executor.GetDefinition()
	for _, param := range definition.Params {
		if param.Name != "credential_id" {
			continue
		}
		if !reflect.DeepEqual(param.CredentialKinds, []string{"API_KEY"}) {
			t.Fatalf("unexpected AI Extract credential kinds: %#v", param.CredentialKinds)
		}
		if !reflect.DeepEqual(param.CredentialProviders, []string{"openai"}) {
			t.Fatalf("unexpected AI Extract credential providers: %#v", param.CredentialProviders)
		}
		return
	}

	t.Fatal("AI Extract credential_id parameter was not found")
}
