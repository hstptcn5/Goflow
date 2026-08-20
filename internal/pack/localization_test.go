package pack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLocalizationValidatesBilingualSetupMetadata(t *testing.T) {
	root := t.TempDir()
	manifest := Manifest{
		Assets: []string{LocalizationAsset},
		ConfigSchema: []ConfigField{
			{Key: "output_language", Label: "Output language", Type: "select", Required: true, Options: []interface{}{"en", "vi"}},
		},
		CredentialRequirements: []CredentialRequirement{{Key: "telegram", Label: "Telegram", Type: "TELEGRAM_BOT", Required: true}},
	}
	content := `{
  "default_locale": "en",
  "locales": {
    "en": {"name":"Daily report","config":{"output_language":{"label":"Output language","options":{"en":"English","vi":"Vietnamese"}}}},
    "vi": {"name":"Báo cáo mỗi ngày","config":{"output_language":{"label":"Ngôn ngữ báo cáo","options":{"en":"Tiếng Anh","vi":"Tiếng Việt"}}},"credentials":{"telegram":{"label":"Mã bot Telegram"}}}
  }
}`
	if err := os.WriteFile(filepath.Join(root, LocalizationAsset), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := LoadLocalization(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.DefaultLocale != "en" || catalog.Locales["vi"].Name != "Báo cáo mỗi ngày" {
		t.Fatalf("unexpected catalog: %#v", catalog)
	}
	if got := catalog.Locales["vi"].Config["output_language"].Options["vi"]; got != "Tiếng Việt" {
		t.Fatalf("unexpected localized option: %q", got)
	}
}

func TestLoadLocalizationRejectsUnknownSetupKey(t *testing.T) {
	root := t.TempDir()
	manifest := Manifest{Assets: []string{LocalizationAsset}}
	content := `{"default_locale":"vi","locales":{"vi":{"config":{"missing":{"label":"Không hợp lệ"}}}}}`
	if err := os.WriteFile(filepath.Join(root, LocalizationAsset), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLocalization(root, manifest)
	if err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("expected unknown localization key error, got %v", err)
	}
}

func TestLoadLocalizationRejectsUnknownSelectOption(t *testing.T) {
	root := t.TempDir()
	manifest := Manifest{
		Assets:       []string{LocalizationAsset},
		ConfigSchema: []ConfigField{{Key: "language", Label: "Language", Type: "select", Required: true, Options: []interface{}{"en", "vi"}}},
	}
	content := `{"default_locale":"en","locales":{"en":{"config":{"language":{"options":{"jp":"Japanese"}}}}}}`
	if err := os.WriteFile(filepath.Join(root, LocalizationAsset), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLocalization(root, manifest)
	if err == nil || !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("expected unknown option error, got %v", err)
	}
}

func TestLoadLocalizationIsOptional(t *testing.T) {
	catalog, err := LoadLocalization(t.TempDir(), Manifest{})
	if err != nil {
		t.Fatal(err)
	}
	if catalog.DefaultLocale != "" || len(catalog.Locales) != 0 {
		t.Fatalf("expected empty optional catalog, got %#v", catalog)
	}
}
