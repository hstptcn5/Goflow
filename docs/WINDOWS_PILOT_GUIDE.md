# Windows DailyOps Pilot Guide

This guide covers the unsigned Goflow DailyOps alpha for Windows x86-64. The native artifact is built and tested on the GitHub Actions `windows-latest` image. It is pilot software, not a signed installer or a production release.

## Download And Verify

1. Download the draft PR's Actions artifact named `UNSIGNED-PILOT-ALPHA-goflow-dailyops-windows-amd64` from the pilot coordinator.
2. Extract the downloaded artifact ZIP to a temporary folder.
3. Confirm that `UNSIGNED-PILOT-ALPHA.txt` says `target=windows-amd64` and lists the expected commit.
4. In PowerShell, run:

```powershell
$expected = (Select-String -Path .\SHA256SUMS.txt -Pattern '  Goflow-DailyOps-Windows-amd64.zip$').Line.Split(' ')[0]
$actual = (Get-FileHash .\Goflow-DailyOps-Windows-amd64.zip -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actual -ne $expected) { throw 'SHA-256 verification failed' }
```

Stop if the hashes differ. Do not run the executable from an artifact that failed verification.

## Extract And Start

1. Move `Goflow-DailyOps-Windows-amd64` to a folder where the pilot user has write access. Keep all files and the `pack` subfolder together.
2. Double-click `goflow.exe` in that folder.
3. Keep the console window open. Goflow opens the appliance UI in the default browser and prints the same local URL in the console.
4. If Microsoft Defender SmartScreen warns about an unrecognized app, confirm that the hash and commit match the pilot handoff. Use **More info** and **Run anyway** only if local policy permits. Do not disable SmartScreen or antivirus protection. Contact the pilot coordinator if policy blocks unsigned software.

## First-Run Setup

1. Enter the absolute HTTP or HTTPS URL for the normalized DailyOps JSON API endpoint. A website homepage, dashboard, or sign-in page is not a source endpoint.
2. Select **Test source** and require `Valid`. The safe summary shows the report date when it has the expected date shape and the number of validated fields; it never displays the full source payload.
3. Enter the Telegram destination chat ID. Send `/start` to the bot first for a direct chat, or add the bot to the target group/channel.
4. Create a Telegram bot credential in the appliance UI. Do not send the token through chat, email, screenshots, issues, or diagnostics.
5. Select **Test Telegram** and require `Valid`. This checks the bot token and chat access without sending a message.
6. Complete setup, then run the managed workflow once with non-production sample data.
7. Keep the page open while `Running...` is displayed. Status and recent executions update automatically.
8. Confirm `SUCCESS` and exactly one Telegram message before using the workflow again.

Changing the source URL or chat ID returns the related test to `Not tested`. Setup completion repeats both checks against the saved current values. A second Run request while DailyOps is active reports `already_running`; wait for the current execution to finish rather than clicking again.

The pilot does not require a real store or production credential. A dedicated non-production Telegram bot and sanitized source data are preferred.

## Data, Backup, And Rotation

Application files remain in the extracted folder. DailyOps runtime state is stored separately at:

```text
%LOCALAPPDATA%\Goflow\packs\official.dailyops-rest-telegram
```

This external directory contains the database, master key, setup state, credential references, and execution history. Follow [Pack Data Backup And Restore](DATA_BACKUP_RESTORE.md) before changing or removing it. Follow [Credential Rotation](CREDENTIAL_ROTATION.md) if a token changes or might have been exposed.

## Stop And Restart

- Stop: focus the Goflow console window and press `Ctrl+C`. Wait for the process to exit before moving files, backing up, or shutting down Windows.
- Restart: double-click the same `goflow.exe`. Goflow reuses the external data directory and the stable managed workflow identity.
- Single instance: starting the same appliance again reuses the running local instance instead of creating another managed workflow.

## Uninstall Completely

1. Stop Goflow with `Ctrl+C` and confirm that `goflow.exe` is no longer running.
2. Delete the extracted `Goflow-DailyOps-Windows-amd64` application folder.
3. Delete `%LOCALAPPDATA%\Goflow\packs\official.dailyops-rest-telegram` to remove the database, master key, setup, credentials, and execution history.
4. Delete any backups or downloaded artifact copies only when they are no longer required by the pilot record.

Removing only the application folder does not remove the external data directory.

## Diagnostics And Feedback

Use the appliance diagnostics view and record the workflow state, execution ID, timestamp, and visible error category. Before sharing diagnostics, screenshots, console output, or logs, remove tokens, credential values, source URLs containing authorization, customer data, local usernames, and filesystem paths. Never attach `goflow.db`, `goflow.master.key`, `.env` files, or the complete data directory.

Report pilot feedback with [Windows Pilot Feedback Template](WINDOWS_PILOT_FEEDBACK_TEMPLATE.md) through the channel provided by the pilot coordinator. Obtain explicit consent before collecting or sharing diagnostics.

## Known Limitations

- The alpha is unsigned, so SmartScreen or organizational policy may block it.
- There is no installer, auto-update, release channel, or automatic uninstall.
- The appliance supports the DailyOps normalized HTTP(S)-to-Telegram example only.
- Pack `0.2.0` removes the unused Report title and Low-stock threshold setup fields. Upgrading preserves the encrypted Telegram credential but requires one setup revalidation before Run is enabled again.
- No store software or vendor integration is claimed compatible unless it has been tested separately.
- The service listens only on the local machine; remote hosting and multi-user operation are outside this pilot.
- Pilot feedback is not evidence of production readiness, customer validation, or willingness to pay.
