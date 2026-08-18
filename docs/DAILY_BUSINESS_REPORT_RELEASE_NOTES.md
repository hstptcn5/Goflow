# Daily Business Report v0.9.0 — Public Beta 1

This prerelease is the first public-distribution build of the productized **Daily Business Report** workflow.

## Fastest path

1. Download `Goflow-Daily-Business-Report-Windows-amd64.zip` and `SHA256SUMS.txt` from this release.
2. Verify the ZIP checksum.
3. Extract the ZIP and run `goflow.exe`.
4. Leave the prefilled public sample source URL unchanged and select **Test source**.
5. Add your Telegram chat ID and bot token, then select **Test Telegram**.
6. Leave AI commentary at `none`, complete setup and select **Run now**.
7. Confirm exactly one Telegram report arrives.
8. Reopen setup and replace the sample URL with your real business-data endpoint.

## What is included

- Goflow runtime and `official.daily-business-report` pack v0.9.0.
- Deterministic first-run reporting with no AI account required.
- Source contract validation before activation.
- Telegram destination connection testing.
- Optional OpenAI or DeepSeek commentary after the deterministic path works.
- Host-managed daily schedule and restart persistence.
- Install guide, changelog, public sample data and SHA-256 checksum manifest.

## Security and integrity

The release pipeline builds the Windows bundle twice and requires deterministic SHA-256 output, verifies both the archive and extracted bundle, and scans distribution output for runtime databases, master keys, saved setup files and test secret material.

The Windows binary is **unsigned public beta software** and may trigger SmartScreen. Verify `SHA256SUMS.txt` before running it.

## Feedback

Use the **Daily Business Report feedback** issue template in this repository. Do not paste Telegram bot tokens, AI API keys, master keys or other secrets into an issue.

This beta does not include payment/licensing, a signed installer or a hosted Goflow control plane.
