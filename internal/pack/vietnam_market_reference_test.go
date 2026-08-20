package pack

import (
	"path/filepath"
	"testing"
)

func TestVietnamMarketPackReferences(t *testing.T) {
	cases := []struct {
		path          string
		id            string
		version       string
		defaultLocale string
	}{
		{"../../examples/packs/daily-business-report", "official.daily-business-report", "0.10.0", "en"},
		{"../../examples/packs/low-stock-alert", "official.low-stock-alert", "0.1.0", "vi"},
		{"../../examples/packs/haravan-zalo-daily-report", "official.haravan-zalo-daily-report", "0.1.0", "vi"},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			loaded, err := Load(filepath.Clean(tc.path))
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Manifest.ID != tc.id || loaded.Manifest.Version != tc.version {
				t.Fatalf("unexpected pack identity %s@%s", loaded.Manifest.ID, loaded.Manifest.Version)
			}
			catalog, err := LoadLocalization(loaded.Root, loaded.Manifest)
			if err != nil {
				t.Fatal(err)
			}
			if catalog.DefaultLocale != tc.defaultLocale {
				t.Fatalf("unexpected default locale %q", catalog.DefaultLocale)
			}
			if _, ok := catalog.Locales["vi"]; !ok {
				t.Fatal("Vietnamese locale is required")
			}
			if _, ok := catalog.Locales["en"]; !ok {
				t.Fatal("English locale is required")
			}
		})
	}
}
