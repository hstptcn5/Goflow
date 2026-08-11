# Windows Pilot Feedback Template

Do not include credentials, tokens, customer data, private source URLs, databases, keys, or unredacted diagnostics.

## Session

- Pilot date:
- Windows version and architecture:
- Goflow commit from `UNSIGNED-PILOT-ALPHA.txt`:
- Installation and first startup successful: Yes / No
- Time from download to first UI load:
- Time from first UI load to first successful workflow run:

## Setup Experience

- Steps that were clear:
- Steps that were confusing or required help:
- SmartScreen or security-policy outcome:
- Credential connection test outcome:
- Source test outcome and whether the JSON API endpoint distinction was clear:
- Telegram bot and chat test outcome, including whether `/start` was needed:
- Workflow outcome and expected Telegram result:
- Running status updated without manually reloading: Yes / No
- Repeated Run click produced only one report: Yes / No / Not tested
- Errors observed, with secrets and local paths removed:

## Persistence

- Goflow stopped cleanly: Yes / No
- Setup and workflow state remained available after restart: Yes / No
- A second launch reused the running instance: Yes / No / Not tested
- Uninstall steps understood: Yes / No

## Current Process

- Software or vendor currently used by the store for this task:
- Current manual steps and approximate time required:
- Would the pilot user use Goflow again for this task: Yes / No / Unsure
- Why:
- Would the pilot user consider paying for a reliable version: Yes / No / Unsure
- Conditions or price assumptions behind that answer:

## Diagnostics Consent

- Diagnostics requested: Yes / No
- Diagnostics were reviewed and redacted with the pilot user: Yes / No / Not applicable
- Pilot user explicitly consents to share the listed diagnostics: Yes / No
- Exact diagnostic items approved for sharing:

Do not share diagnostics when explicit consent is `No` or missing.
