package apperror

import "testing"

func TestPublicCategoryAllowlist(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"source_invalid_json", CategorySourceInvalid, true},
		{"source_unreachable", CategorySourceUnreachable, true},
		{"source_timeout", CategorySourceTimeout, true},
		{"source_contract_invalid", CategorySourceContractMismatch, true},
		{"telegram_unauthorized", CategoryTelegramBotUnauthorized, true},
		{"telegram_chat_inaccessible", CategoryTelegramChatNotFound, true},
		{"already_running", CategoryAlreadyRunning, true},
		{"schedule_invalid", CategoryScheduleInvalid, true},
		{"schedule_missed_skipped", CategoryScheduleMissedSkipped, true},
		{"config", CategoryMigrationRequired, true},
		{"revalidation", CategoryRevalidationRequired, true},
		{"tamper_detected", CategoryArtifactTamper, true},
		{"legacy-secret-token", "", false},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, message, ok := Public(test.input)
			if got != test.want || ok != test.ok {
				t.Fatalf("Public(%q) = %q, %t; want %q, %t", test.input, got, ok, test.want, test.ok)
			}
			if ok && message == "" {
				t.Fatal("public category has no fixed message")
			}
		})
	}
}
