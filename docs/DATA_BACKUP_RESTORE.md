# Pack Data Backup And Restore

Runtime state is stored outside source packs and extracted bundles.

Default locations:

- Windows: `%LOCALAPPDATA%/Goflow/packs/<pack-id>/`
- macOS: `~/Library/Application Support/Goflow/packs/<pack-id>/`
- Linux: `$XDG_DATA_HOME/Goflow/packs/<pack-id>/` or `~/.local/share/Goflow/packs/<pack-id>/`

The data directory contains:

- `goflow.db`
- `goflow.master.key`
- `pack-state.json`
- `run-state.json`
- `pack-config.json`
- `pack-credentials.json`
- lock metadata

## Backup

1. Stop `pack run`.
2. Copy the entire pack data directory.
3. Store `goflow.db` and `goflow.master.key` together. Credentials cannot be decrypted without the matching master key.
4. Keep the backup outside source control.

## Restore

1. Stop `pack run`.
2. Restore the complete data directory to the same pack ID path or pass it with `--data-dir`.
3. Start `pack run` again.
4. Open the appliance dashboard and confirm setup readiness.

## Rotation And Recovery

If `goflow.master.key` is lost, encrypted credentials in `goflow.db` are not recoverable. Create new credentials through the appliance setup UI and re-run connection tests.
