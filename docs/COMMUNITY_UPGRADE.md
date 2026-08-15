# Community RC Upgrade Guide

This guide covers evaluation of Goflow `1.0.0-rc.1` from the merged Beta base
`66fe7993f99aeac32339324dd07f58f47ad1f940`.

1. Stop Goflow cleanly. Do not copy a live SQLite database.
2. Locate the external data directory containing `goflow.db` and
   `goflow.master.key`.
3. Copy the complete directory to two safe locations. Keep the database and its
   matching master key together, and never overwrite the only backup.
4. Download the candidate from the expected exact-head GitHub Actions run.
5. Verify the outer `.sha256` file, then inspect `COMMUNITY_ARTIFACT.json` for
   marker `UNSIGNED-COMMUNITY-RC`, version, channel, exact commit, and target.
6. Extract the candidate into a new empty application directory. Do not extract
   it over the previous runtime.
7. Copy the backup to a disposable test data directory and configure
   `GOFLOW_DB_PATH` and `GOFLOW_MASTER_KEY_FILE` to use that copy.
8. Start the candidate on loopback, confirm `/healthz`, open the UI, and verify
   representative workflows and credentials are present without duplication.
9. Only after that test succeeds, repeat against the intended data directory.

For rollback, stop the candidate and restore both the previous runtime and a
complete pre-upgrade data-directory backup. Do not point an older runtime at a
database already migrated by a newer version. Goflow intentionally fails closed
when it sees an unknown future migration; it does not provide automatic
downgrade conversion.

See [Pack data backup and restore](DATA_BACKUP_RESTORE.md) for Pack-specific
locations. Generic platform operators choose the database and key paths with
`GOFLOW_DB_PATH` and `GOFLOW_MASTER_KEY_FILE`.
