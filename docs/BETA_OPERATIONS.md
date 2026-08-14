# DailyOps Beta Operations Package

Use these documents for the unsigned Windows beta pilot:

- [Windows pilot guide](WINDOWS_PILOT_GUIDE.md): verification, extraction,
  SmartScreen, setup, stop/restart, offline upgrade/rollback, and uninstall.
- [Source JSON contract](BETA_SOURCE_JSON_CONTRACT.md).
- [Telegram setup](BETA_TELEGRAM_SETUP.md).
- [Schedule and timezone](BETA_SCHEDULE_TIMEZONE.md).
- [Troubleshooting matrix](APPLIANCE_TROUBLESHOOTING.md).
- [Backup and data location](DATA_BACKUP_RESTORE.md).
- [Credential rotation](CREDENTIAL_ROTATION.md).
- [Diagnostics and privacy](productization/DIAGNOSTICS.md).
- [Consent and feedback template](WINDOWS_PILOT_FEEDBACK_TEMPLATE.md).
- [Support response template](BETA_SUPPORT_RESPONSE_TEMPLATE.md).
- [Beta limitations](BETA_LIMITATIONS.md).
- [Manual acceptance checklist](BETA_MANUAL_ACCEPTANCE_CHECKLIST.md).

## Local Or User-Recorded Metrics

Record setup success or public failure category, user-observed time to first
success, manual/scheduled success and failure counts, duplicate prevention,
restart/update outcome, report usefulness, and willingness to continue/pay.
Runtime diagnostics export only bounded status, duration, and public category
summaries. Do not collect raw source payloads, tokens, chat IDs, personal
content, host identity, or filesystem paths. Goflow sends no pilot telemetry.

