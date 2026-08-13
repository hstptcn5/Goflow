# Pack Author Tutorial

This tutorial uses only local files and the Goflow CLI. The scaffolded pack contains no credentials or example secret values, and includes a bounded author-only `tests/offline.json` fixture.

## 1. Initialize

PowerShell:

```powershell
.\goflow.exe pack init .\my-pack --id example.my-pack --name "My Pack"
```

POSIX:

```sh
./goflow pack init ./my-pack --id example.my-pack --name "My Pack"
```

`pack init` refuses a non-empty directory unless `--force` is supplied. The generated files are written through a temporary scaffold and published only after validation succeeds.

## 2. Validate

PowerShell:

```powershell
.\goflow.exe pack validate .\my-pack
```

POSIX:

```sh
./goflow pack validate ./my-pack
```

Validation checks `pack.json`, the entry workflow, portable paths, setup metadata, bindings, supported platform metadata, required runtime capabilities, the offline fixture, and pack-only secret rules.

## 3. Inspect

PowerShell:

```powershell
.\goflow.exe pack inspect .\my-pack --output json
```

POSIX:

```sh
./goflow pack inspect ./my-pack --output json
```

Inspect reports identity, version, target/capability support, setup counts, controlled files, plugin/asset counts, fixture presence, and integrity state. It does not print workflow parameter or fixture values.

## 4. Offline Test

PowerShell:

```powershell
.\goflow.exe pack test .\my-pack --output json
```

POSIX:

```sh
./goflow pack test ./my-pack --output json
```

The offline test validates the pack, overlays the strict fixture on deterministic non-secret setup config, binds fake credential IDs for declared credential slots, applies setup bindings to a cloned workflow, and prepares the managed workflow twice in a temporary data directory. It does not open a browser, call the network, run external programs, or send messages. Connection tests that require a real external service are reported as skipped.

The fixture schema is `{ "schema_version": 1, "config": { ... } }`. Unknown fields or config keys, path escapes, symlinks, oversized files, future schemas, secret-like values, and values outside `config_schema` fail closed. The fixture and its manifest reference are excluded from the runtime bundle.

## 5. Build

PowerShell:

```powershell
.\goflow.exe pack build .\my-pack --output .\dist
```

POSIX:

```sh
./goflow pack build ./my-pack --output ./dist
```

Builds are deterministic for the same input files and target runtime. The archive includes `PACK_INFO.json`, a runtime entry, the runtime Pack payload, and generated run instructions. Every shipped file is inventoried; undeclared author files and the offline fixture are not shipped. Bundles are unsigned; verification checks local integrity against `PACK_INFO.json`.

## 6. Extract And Verify

PowerShell:

```powershell
Expand-Archive .\dist\example.my-pack-0.1.0-windows-amd64.zip .\dist\example.my-pack
.\goflow.exe pack verify .\dist\example.my-pack
.\goflow.exe pack verify .\dist\example.my-pack-0.1.0-windows-amd64.zip --output json
```

POSIX:

```sh
unzip ./dist/example.my-pack-0.1.0-linux-amd64.zip -d ./dist/example.my-pack
./goflow pack verify ./dist/example.my-pack
./goflow pack verify ./dist/example.my-pack-0.1.0-linux-amd64.zip --output json
```

`pack verify` does not run or import the pack. It verifies ZIP inventory or extracted-bundle inventory and checks hashes, expected files, and pack metadata.

## 7. Run Locally

PowerShell:

```powershell
.\goflow.exe pack run .\my-pack --data-dir .\pack-data --no-open
```

POSIX:

```sh
./goflow pack run ./my-pack --data-dir ./pack-data --no-open
```

Pack Run starts a loopback-only appliance server and prints the URL. Setup values are stored in the data directory. Credentials are encrypted in the credential store; decrypted values are never written to `pack.json`, workflow JSON, diagnostics, URLs, or built bundles.

## Setup Metadata

Use `config_schema` for non-secret setup values such as titles, source URLs, thresholds, and chat IDs. Use `credential_requirements` for secrets such as bot tokens, API keys, SSH keys, OAuth tokens, and database URLs. Bind setup values into workflow node parameters through `bindings`.

URL defaults must be absolute `http` or `https` URLs. Credential `test_kind` values are closed by credential type. Unsupported or impossible combinations are rejected by validation.

Declare compatibility with the optional closed `required_capabilities` list:
`goflow.pack.v1`, `goflow.setup.bindings.v1`,
`goflow.setup.connection-tests.v1`, `goflow.schedule.daily.v1`, and
`goflow.migration.host-managed.v1`. Unknown capabilities fail clearly. Legacy
Pack Format v1 manifests may omit the list. Raw appliance cron and Pack-provided
migration code remain unsupported.

## Secret Rules

Do not put secrets in manifest fields, workflow parameters, fixture data, README examples, bundle paths, URLs, logs, or diagnostics. The pack validator rejects known secret-bearing workflow parameters in pack context. Use credential slots and encrypted credential storage instead.
