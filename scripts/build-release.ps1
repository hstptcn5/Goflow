param(
  [string]$VersionFile = "VERSION"
)

$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

$version = (Get-Content $VersionFile -Raw).Trim()
if (-not $version) {
  throw "VERSION is empty"
}

$releaseRoot = Join-Path $root "release"
$stage = Join-Path $releaseRoot "goflow-$version-windows-amd64"
$archive = Join-Path $releaseRoot "goflow-$version-windows-amd64.zip"

if (Test-Path $stage) {
  Remove-Item -LiteralPath $stage -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $stage | Out-Null
New-Item -ItemType Directory -Force -Path $releaseRoot | Out-Null

Push-Location "ui"
npm ci
if ($LASTEXITCODE -ne 0) { throw "npm ci failed with exit code $LASTEXITCODE" }
npm run build
if ($LASTEXITCODE -ne 0) { throw "npm run build failed with exit code $LASTEXITCODE" }
Pop-Location

go test ./...
if ($LASTEXITCODE -ne 0) { throw "go test failed with exit code $LASTEXITCODE" }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw "go vet failed with exit code $LASTEXITCODE" }
go build -trimpath -ldflags="-s -w" -o (Join-Path $stage "goflow.exe") main.go static_embed.go
if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }

Copy-Item README.md, NODES.md, PLUGINS.md, BACKUP.md, ROADMAP.md, CLI_MCP_ROADMAP.md, ROADMAP_PROGRESS.md, CHANGELOG.md, COMMERCIAL.md, TRADEMARK.md, LICENSE, VERSION -Destination $stage
Copy-Item plugins -Destination $stage -Recurse
Copy-Item templates -Destination $stage -Recurse

if (Test-Path $archive) {
  Remove-Item -LiteralPath $archive -Force
}
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $archive

$hash = Get-FileHash $archive -Algorithm SHA256
$hash.Hash | Out-File "$archive.sha256" -Encoding ascii

Write-Host "Created $archive"
Write-Host "SHA256 $($hash.Hash)"
