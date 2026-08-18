package pack

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	LocalizationAsset    = "locales.json"
	MaxLocalizationBytes = 64 << 10
	MaxLocales           = 8
)

type LocalizedSetupText struct {
	Label       string            `json:"label,omitempty"`
	Description string            `json:"description,omitempty"`
	Options     map[string]string `json:"options,omitempty"`
}

type LocaleContent struct {
	Name        string                        `json:"name,omitempty"`
	Description string                        `json:"description,omitempty"`
	Config      map[string]LocalizedSetupText `json:"config,omitempty"`
	Credentials map[string]LocalizedSetupText `json:"credentials,omitempty"`
}

type LocalizationCatalog struct {
	DefaultLocale string                   `json:"default_locale"`
	Locales       map[string]LocaleContent `json:"locales"`
}

func LoadLocalization(root string, manifest Manifest) (LocalizationCatalog, error) {
	if !manifestHasAsset(manifest, LocalizationAsset) {
		return LocalizationCatalog{}, nil
	}
	path, err := resolveExistingRegularInside(root, LocalizationAsset)
	if err != nil {
		return LocalizationCatalog{}, fmt.Errorf("localization: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return LocalizationCatalog{}, fmt.Errorf("localization: open %s: %w", LocalizationAsset, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxLocalizationBytes+1))
	if err != nil {
		return LocalizationCatalog{}, fmt.Errorf("localization: read %s: %w", LocalizationAsset, err)
	}
	if len(data) > MaxLocalizationBytes {
		return LocalizationCatalog{}, fmt.Errorf("localization: %s exceeds %d byte limit", LocalizationAsset, MaxLocalizationBytes)
	}
	var catalog LocalizationCatalog
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalog); err != nil {
		return LocalizationCatalog{}, fmt.Errorf("localization: %s must be valid JSON: %w", LocalizationAsset, err)
	}
	if err := validateLocalization(catalog, manifest); err != nil {
		return LocalizationCatalog{}, err
	}
	return catalog, nil
}

func manifestHasAsset(manifest Manifest, name string) bool {
	for _, asset := range manifest.Assets {
		if asset == name {
			return true
		}
	}
	return false
}

func validateLocalization(catalog LocalizationCatalog, manifest Manifest) error {
	if len(catalog.Locales) == 0 {
		return fmt.Errorf("localization: locales must include at least one locale")
	}
	if len(catalog.Locales) > MaxLocales {
		return fmt.Errorf("localization: locales exceeds %d item limit", MaxLocales)
	}
	if !validLocaleKey(catalog.DefaultLocale) {
		return fmt.Errorf("localization: default_locale must be a supported locale key")
	}
	if _, ok := catalog.Locales[catalog.DefaultLocale]; !ok {
		return fmt.Errorf("localization: default_locale %q is not present in locales", catalog.DefaultLocale)
	}
	config := make(map[string]ConfigField, len(manifest.ConfigSchema))
	for _, field := range manifest.ConfigSchema {
		config[field.Key] = field
	}
	credentials := make(map[string]CredentialRequirement, len(manifest.CredentialRequirements))
	for _, req := range manifest.CredentialRequirements {
		credentials[req.Key] = req
	}
	for locale, content := range catalog.Locales {
		if !validLocaleKey(locale) {
			return fmt.Errorf("localization: locale key %q is invalid", locale)
		}
		if err := validateLocalizedText(content.Name, "localization: locales."+locale+".name", false, MaxSetupLabelLength); err != nil {
			return err
		}
		if err := validateLocalizedText(content.Description, "localization: locales."+locale+".description", false, MaxSetupDescriptionLength); err != nil {
			return err
		}
		for key, text := range content.Config {
			field, ok := config[key]
			if !ok {
				return fmt.Errorf("localization: locales.%s.config references unknown key %q", locale, key)
			}
			if err := validateLocalizedSetupText(text, fmt.Sprintf("localization: locales.%s.config.%s", locale, key)); err != nil {
				return err
			}
			if len(text.Options) > 0 {
				if field.Type != "select" {
					return fmt.Errorf("localization: locales.%s.config.%s.options is allowed only for select fields", locale, key)
				}
				allowed := map[string]bool{}
				for _, option := range field.Options {
					allowed[fmt.Sprint(option)] = true
				}
				for option, label := range text.Options {
					if !allowed[option] {
						return fmt.Errorf("localization: locales.%s.config.%s.options references unknown option %q", locale, key, option)
					}
					if err := validateLocalizedText(label, fmt.Sprintf("localization: locales.%s.config.%s.options.%s", locale, key, option), true, MaxSetupOptionLength); err != nil {
						return err
					}
				}
			}
		}
		for key, text := range content.Credentials {
			if _, ok := credentials[key]; !ok {
				return fmt.Errorf("localization: locales.%s.credentials references unknown key %q", locale, key)
			}
			if len(text.Options) > 0 {
				return fmt.Errorf("localization: credential text must not declare options")
			}
			if err := validateLocalizedSetupText(text, fmt.Sprintf("localization: locales.%s.credentials.%s", locale, key)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLocalizedSetupText(text LocalizedSetupText, prefix string) error {
	if err := validateLocalizedText(text.Label, prefix+".label", false, MaxSetupLabelLength); err != nil {
		return err
	}
	return validateLocalizedText(text.Description, prefix+".description", false, MaxSetupDescriptionLength)
}

func validateLocalizedText(value, field string, required bool, max int) error {
	trimmed := strings.TrimSpace(value)
	if required && trimmed == "" {
		return fmt.Errorf("%s is required", field)
	}
	if len(value) > max {
		return fmt.Errorf("%s exceeds %d character limit", field, max)
	}
	return nil
}

func validLocaleKey(locale string) bool {
	if len(locale) < 2 || len(locale) > 20 {
		return false
	}
	for i, r := range locale {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9') || (i > 0 && r == '-') {
			continue
		}
		return false
	}
	return true
}
