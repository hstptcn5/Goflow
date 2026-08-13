package apperror

const (
	CategorySourceInvalid           = "source_invalid"
	CategorySourceUnreachable       = "source_unreachable"
	CategorySourceTimeout           = "source_timeout"
	CategorySourceContractMismatch  = "source_contract_mismatch"
	CategoryTelegramBotUnauthorized = "telegram_bot_unauthorized"
	CategoryTelegramChatNotFound    = "telegram_chat_not_found"
	CategoryTelegramUnreachable     = "telegram_unreachable"
	CategoryAlreadyRunning          = "already_running"
	CategoryScheduleInvalid         = "schedule_invalid"
	CategoryScheduleMissedSkipped   = "schedule_missed_skipped"
	CategoryMigrationRequired       = "migration_required"
	CategoryRevalidationRequired    = "revalidation_required"
	CategoryArtifactTamper          = "artifact_tamper"
	CategorySetupIncomplete         = "setup_incomplete"
	CategoryRateLimited             = "rate_limited"
	CategoryRevisionConflict        = "revision_conflict"
	CategoryCancelled               = "cancelled"
	CategoryInterrupted             = "interrupted"
	CategoryInternal                = "internal_error"
)

// Public maps internal and legacy categories onto the closed diagnostics/API
// vocabulary. Messages are fixed here so persisted legacy text cannot leak.
func Public(category string) (string, string, bool) {
	switch category {
	case "source_invalid_url", "source_non_json", "source_invalid_json", "source_response_too_large":
		return CategorySourceInvalid, "The source configuration or response is invalid.", true
	case "source_http_error", CategorySourceUnreachable:
		return CategorySourceUnreachable, "Goflow could not reach a usable source endpoint.", true
	case CategorySourceTimeout:
		return CategorySourceTimeout, "The source request timed out.", true
	case "source_contract_invalid", CategorySourceContractMismatch:
		return CategorySourceContractMismatch, "The source data does not match the required contract.", true
	case "telegram_unauthorized", CategoryTelegramBotUnauthorized:
		return CategoryTelegramBotUnauthorized, "Telegram rejected the configured bot credential.", true
	case "telegram_chat_inaccessible", CategoryTelegramChatNotFound:
		return CategoryTelegramChatNotFound, "Telegram could not access the configured destination.", true
	case CategoryTelegramUnreachable:
		return CategoryTelegramUnreachable, "Goflow could not reach Telegram.", true
	case "test_already_running", CategoryAlreadyRunning:
		return CategoryAlreadyRunning, "The operation is already running.", true
	case CategoryScheduleInvalid:
		return CategoryScheduleInvalid, "The saved schedule is invalid.", true
	case "workflow_inactive":
		return CategoryScheduleInvalid, "The managed workflow is inactive.", true
	case CategoryScheduleMissedSkipped:
		return CategoryScheduleMissedSkipped, "A missed scheduled run was skipped.", true
	case "config", "user_review", CategoryMigrationRequired:
		return CategoryMigrationRequired, "The Pack update requires setup review.", true
	case "revalidation", CategoryRevalidationRequired:
		return CategoryRevalidationRequired, "The Pack update requires revalidation.", true
	case "artifact_invalid", "tamper_detected", CategoryArtifactTamper:
		return CategoryArtifactTamper, "Pack integrity verification failed.", true
	case CategorySetupIncomplete:
		return CategorySetupIncomplete, "Setup is incomplete.", true
	case CategoryRateLimited:
		return CategoryRateLimited, "The operation is temporarily rate limited.", true
	case CategoryRevisionConflict:
		return CategoryRevisionConflict, "The saved value changed in another session.", true
	case CategoryCancelled:
		return CategoryCancelled, "The workflow was cancelled.", true
	case CategoryInterrupted:
		return CategoryInterrupted, "The workflow stopped before it completed.", true
	case CategoryInternal:
		return CategoryInternal, "The operation could not be completed.", true
	default:
		return "", "", false
	}
}
