# Windows DailyOps Pilot Guide

This guide covers the unsigned Goflow DailyOps beta candidate for Windows x86-64. The native artifact is built and tested on the GitHub Actions `windows-latest` image. It is pilot software, not a signed installer or a production release.

## Download And Verify

1. Download the draft PR's Actions artifact named `UNSIGNED-PILOT-BETA-goflow-dailyops-windows-amd64` from the pilot coordinator.
2. Extract the downloaded artifact ZIP to a temporary folder.
3. Confirm that `UNSIGNED-PILOT-BETA.txt` says `target=windows-amd64` and lists the expected commit.
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

## User-Approved Offline Upgrade

There is no automatic download or silent update. Obtain the new unsigned beta
artifact through the pilot coordinator, verify its `SHA256SUMS.txt`, and extract
the outer artifact to a separate folder. Do not overwrite the running app.

1. Stop Goflow with `Ctrl+C` and wait for `goflow.exe` to exit.
2. Back up the external data directory using [Pack Data Backup And Restore](DATA_BACKUP_RESTORE.md).
3. From the newly extracted artifact folder, run PowerShell with the current
   application folder and the new candidate bundle:

```powershell
.\Update-Goflow.ps1 `
  -ApplicationDirectory 'C:\Apps\Goflow-DailyOps-Windows-amd64' `
  -CandidateBundle '.\Goflow-DailyOps-Windows-amd64.zip'
```

The helper performs no network request. It rejects a running instance, verifies
the candidate ZIP and extracted inventory with `goflow.exe pack verify`, checks
the Pack identity and Windows AMD64 target, snapshots external data, retains the
old app directory, activates the candidate, and waits for local `/healthz`.
Tamper, startup, or health failure restores the prior app and data. A successful
update prints the rollback directory paths and may require source/Telegram
revalidation before manual or scheduled execution resumes.

Keep the `.rollback-<timestamp>` app/data directories until the updated
appliance has completed setup revalidation, one manual run, one scheduled run,
and a restart. To recover manually, stop Goflow, move the failed app/data aside,
restore those rollback directories to their original names, and start the prior
`goflow.exe`. Never combine app files from two candidates.

## Uninstall Completely

1. Stop Goflow with `Ctrl+C` and confirm that `goflow.exe` is no longer running.
2. Delete the extracted `Goflow-DailyOps-Windows-amd64` application folder.
3. Delete `%LOCALAPPDATA%\Goflow\packs\official.dailyops-rest-telegram` to remove the database, master key, setup, credentials, and execution history.
4. Delete any backups or downloaded artifact copies only when they are no longer required by the pilot record.

Removing only the application folder does not remove the external data directory.

## Diagnostics And Feedback

Use **Copy diagnostics** or **Download diagnostics** in the local pilot summary.
The bounded export omits credential/workflow/execution IDs, timestamps, source
URLs, chat IDs, payloads, logs, and filesystem paths. Still review any
screenshots, console output, or additional logs before sharing. Never attach
`goflow.db`, `goflow.master.key`, `.env` files, or the complete data directory.

Report pilot feedback with [Windows Pilot Feedback Template](WINDOWS_PILOT_FEEDBACK_TEMPLATE.md) through the channel provided by the pilot coordinator. Obtain explicit consent before collecting or sharing diagnostics.

## Known Limitations

- The beta candidate is unsigned, so SmartScreen or organizational policy may block it.
- There is no installer, auto-update, release channel, signing, or automatic uninstall. `BLOCKED_INSTALLER_TOOLCHAIN` remains until a compiler/SDK and signing path are pinned and reviewed.
- The appliance supports the DailyOps normalized HTTP(S)-to-Telegram example only.
- Pack `0.3.0` includes the host-managed 0.1 -> 0.2 -> 0.3 setup migration path. Upgrading preserves the encrypted Telegram credential and schedule preference but requires bounded setup revalidation before execution resumes.
- No store software or vendor integration is claimed compatible unless it has been tested separately.
- The service listens only on the local machine; remote hosting and multi-user operation are outside this pilot.
- Pilot feedback is not evidence of production readiness, customer validation, or willingness to pay.
