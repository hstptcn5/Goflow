# Goflow Backup and Restore Guide

This guide explains what to back up, how to restore a Goflow instance, and what can go wrong if the encryption master key is lost.

Vietnamese: Tai lieu nay huong dan backup/restore Goflow, dac biet la database SQLite va master key dung de giai ma credentials.

---

## What to Back Up

Back up these files and settings together:

| Item | Why it matters |
| :--- | :--- |
| `goflow.db` | Main SQLite database containing workflows, executions, credentials metadata, and settings. |
| `goflow.db-wal` and `goflow.db-shm` | SQLite WAL files. Include them if the app is running during backup. |
| `goflow.master.key` | Local encryption key file created when `GOFLOW_MASTER_KEY` is not set. Required to decrypt saved credentials. |
| `GOFLOW_MASTER_KEY` or `GOFLOW_MASTER_KEY_FILE` | If you use environment-based key management, back up the secret outside the project folder. |
| Environment variables | Includes `GOFLOW_API_KEY`, host/port, retention settings, rate limits, OAuth settings, and other deployment config. |
| Workflow exports/templates | Optional but useful for recovery, migration, and sharing. Export important workflows from the UI. |
| Binary version | Keep the exact Goflow release or commit used by the instance. |

Vietnamese:

- Backup `goflow.db` de giu workflows, executions va credentials da ma hoa.
- Backup `goflow.master.key` hoac bien moi truong `GOFLOW_MASTER_KEY`. Neu mat key nay, credentials da luu se khong giai ma duoc.
- Neu backup khi Goflow dang chay, backup ca `goflow.db-wal` va `goflow.db-shm`.

---

## Recommended Backup Method

The safest simple backup is:

1. Stop Goflow.
2. Copy `goflow.db`.
3. Copy `goflow.master.key` or securely record the configured master key.
4. Start Goflow again.

This avoids partial SQLite WAL state.

### Windows PowerShell

```powershell
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$backupDir = "backups\goflow-$stamp"
New-Item -ItemType Directory -Force -Path $backupDir

Copy-Item .\goflow.db $backupDir\
if (Test-Path .\goflow.db-wal) { Copy-Item .\goflow.db-wal $backupDir\ }
if (Test-Path .\goflow.db-shm) { Copy-Item .\goflow.db-shm $backupDir\ }
if (Test-Path .\goflow.master.key) { Copy-Item .\goflow.master.key $backupDir\ }

Get-ChildItem Env:GOFLOW_* | Out-File "$backupDir\goflow-env.txt"
```

### Linux / macOS / Bash

```bash
stamp="$(date +%Y%m%d-%H%M%S)"
backup_dir="backups/goflow-$stamp"
mkdir -p "$backup_dir"

cp goflow.db "$backup_dir/"
[ -f goflow.db-wal ] && cp goflow.db-wal "$backup_dir/"
[ -f goflow.db-shm ] && cp goflow.db-shm "$backup_dir/"
[ -f goflow.master.key ] && cp goflow.master.key "$backup_dir/"

env | grep '^GOFLOW_' > "$backup_dir/goflow-env.txt"
```

---

## Restore

1. Stop Goflow.
2. Copy the backed-up `goflow.db` into the Goflow working directory.
3. Copy the matching `goflow.master.key`, or restore the same `GOFLOW_MASTER_KEY` / `GOFLOW_MASTER_KEY_FILE`.
4. Restore required environment variables such as `GOFLOW_API_KEY`, host/port, and OAuth settings.
5. Start Goflow.
6. Open the UI and verify workflows, credentials, and recent executions.

Vietnamese:

1. Tat Goflow.
2. Chep lai `goflow.db`.
3. Chep dung `goflow.master.key` hoac cau hinh lai dung `GOFLOW_MASTER_KEY`.
4. Khoi phuc cac bien moi truong can thiet.
5. Chay lai Goflow va kiem tra workflows/credentials.

---

## Credential Recovery Warning

Encrypted credentials require the same master key that was used when they were saved.

If the database is restored without the matching master key:

- Workflows still exist.
- Execution history still exists.
- Saved credential records still exist.
- Secret values cannot be decrypted.
- Affected credentials must be recreated manually.

Vietnamese: Neu mat master key, khong co cach giai ma lai API key/token da luu. Cach phuc hoi thuc te la tao lai credentials moi.

---

## Suggested Schedule

For personal or small internal deployments:

- Daily: database and master key backup.
- Weekly: full folder backup including workflow templates and release binary.
- Before upgrades: backup database, master key, and current executable.
- After adding important credentials: run an immediate backup.

Keep at least one backup outside the server or VM that runs Goflow.

