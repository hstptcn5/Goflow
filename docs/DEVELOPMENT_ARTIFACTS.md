# Development Artifact Verification

Goflow CI artifacts are temporary, unsigned evaluation outputs tied to an exact
commit and workflow run. They are not GitHub Releases, installers, signed
distribution, or latest-version pointers.

## Expected CI Behavior

Full pull-request CI for code or workflow changes runs:

- Backend tests, race tests, vet, and `govulncheck`.
- Frontend unit tests and production build.
- Scoped appliance Playwright E2E and runner smoke.
- Pack CLI contracts.
- DailyOps offline mock test.
- Deterministic bundle comparison.
- Bundle verification and canary/path scan.
- Multi-platform runtime build matrix.

For code or workflow changes, full CI also verifies and uploads the native
Windows DailyOps artifact:

```text
UNSIGNED-PILOT-BETA-goflow-dailyops-windows-amd64
```

This artifact is an unsigned Productization Beta pilot candidate. It is not a
stable platform release or a claim of vendor compatibility. Docs-only pull
requests use the lightweight documentation path and do not build it.

Manual `workflow_dispatch` additionally uploads artifacts named:

```text
UNSIGNED-DEVELOPMENT-ALPHA-goflow-<goos>-<goarch>
```

The manual workflow must not create tags, GitHub Releases, installers, signatures, or latest-version pointers.

The Alpha marker remains the name of this separate development channel; it does
not imply that the current product status has reverted from Productization
Beta. Do not use `workflow_dispatch` merely to obtain a general release.

## Verify An Artifact

1. Download the artifact from the exact GitHub Actions run.
2. Confirm the exact expected channel: `UNSIGNED-PILOT-BETA` for the native
   Windows pilot or `UNSIGNED-DEVELOPMENT-ALPHA` for manual multi-platform
   development artifacts.
3. Extract it into a temporary directory.
4. Check the matching `UNSIGNED-PILOT-BETA.txt` or
   `UNSIGNED-DEVELOPMENT-ALPHA.txt` metadata and `SHA256SUMS-*.txt`.
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
