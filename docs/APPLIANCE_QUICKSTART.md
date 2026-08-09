# Goflow Appliance Alpha Quickstart

This guide is for a local alpha appliance pack such as `official.dailyops-rest-telegram`.

## What You Need

- An unsigned Goflow development artifact for your OS/CPU.
- A source pack directory or extracted pack bundle.
- Non-production sample data.
- A Telegram bot token only when you intentionally test Telegram delivery.

Do not paste production secrets into chat, GitHub issues, docs, or committed files.

## Start The Appliance

From a terminal:

```powershell
.\goflow-windows-amd64.exe pack run examples\packs\dailyops-rest-telegram --no-open
```

Goflow prints a loopback URL such as:

```text
Pack running
URL: http://127.0.0.1:53177/workflows
Data directory: C:\Users\<you>\AppData\Local\Goflow\packs\official.dailyops-rest-telegram
Workflow ID: edb51040-bac0-5c0d-9247-e770ec52e418
```

Open the printed URL in the same browser. Appliance mode is local-only and binds to `127.0.0.1`.

## First-Run Setup

The appliance UI reads `/api/appliance/bootstrap` and shows setup when the pack declares setup metadata.

Complete:

- Non-secret config values such as source URL, chat ID, report title, and threshold.
- Credential creation for required slots.
- Connection test when a supported non-mutating test exists.
- Setup completion.

Credential values are sent to the local appliance credential endpoint and stored through the encrypted credential store. They are not written into `pack.json`, workflow JSON, bundle files, or diagnostics.

## Run The Workflow

After setup is ready:

- Use the appliance dashboard run button.
- Review latest and recent execution summaries.
- Use diagnostics only for support-safe state summaries.

The dashboard does not expose decrypted credentials, credential IDs, raw workflow IO, database paths, or master key paths.

## Stop

Stop the terminal process with `Ctrl+C`. Runtime state remains in the pack data directory so the next run can reuse config, credentials, workflow ID, and execution history.

## Alpha Limits

- Artifacts are unsigned development builds, not production releases.
- `PACK_INFO.json` is integrity metadata, not publisher identity.
- Packaged plugin/native execution is blocked.
- Generic `goflow serve` does not mount appliance routes.
- Current reference pack tests are mock/local unless you supply your own non-production credentials.
