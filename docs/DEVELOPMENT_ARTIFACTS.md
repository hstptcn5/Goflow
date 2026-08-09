# Development Artifact Verification

Goflow ecosystem alpha artifacts are unsigned development artifacts. They are for pilot verification only.

## Expected CI Behavior

Pull-request CI runs:

- Backend tests, race tests, vet, and `govulncheck`.
- Frontend unit tests and production build.
- Scoped appliance Playwright E2E and runner smoke.
- Pack CLI contracts.
- DailyOps offline mock test.
- Deterministic bundle comparison.
- Bundle verification and canary/path scan.
- Multi-platform runtime build matrix.

Manual `workflow_dispatch` additionally uploads artifacts named:

```text
UNSIGNED-DEVELOPMENT-ALPHA-goflow-<goos>-<goarch>
```

The manual workflow must not create tags, GitHub Releases, installers, signatures, or latest-version pointers.

## Verify An Artifact

1. Download the artifact from the exact GitHub Actions run.
2. Confirm the artifact name includes `UNSIGNED-DEVELOPMENT-ALPHA` for manual alpha artifacts.
3. Extract it into a temporary directory.
4. Check the included `UNSIGNED-DEVELOPMENT-ALPHA.txt` metadata and `SHA256SUMS-*.txt`.
5. Run the binary only on a local trusted machine.
6. For pack bundles, run:

   ```bash
   goflow pack verify <bundle.zip>
   goflow pack verify <extracted-bundle-directory>
   ```

7. Scan extracted members for unexpected runtime state:

   - `goflow.db`
   - `*.key`
   - `.env`
   - `pack-config.json`
   - `pack-credentials.json`
   - concrete local build paths or seeded canary values

## Windows Smoke

After extraction, start a source pack with a temp data directory:

```powershell
.\goflow-windows-amd64.exe pack run examples\packs\dailyops-rest-telegram --data-dir $env:TEMP\goflow-smoke-data --port 53177 --no-open
```

Then verify:

- `http://127.0.0.1:53177/api/appliance/bootstrap` returns the DailyOps pack ID and a token.
- `http://127.0.0.1:53177/` serves the embedded UI shell.

Stop the process and delete the temp data directory.
