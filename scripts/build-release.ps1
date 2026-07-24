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
npm run build
Pop-Location

go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w" -o (Join-Path $stage "goflow.exe") main.go static_embed.go

Copy-Item README.md, NODES.md, BACKUP.md, CHANGELOG.md, COMMERCIAL.md, TRADEMARK.md, LICENSE, VERSION -Destination $stage
Copy-Item templates -Destination $stage -Recurse

if (Test-Path $archive) {
  Remove-Item -LiteralPath $archive -Force
}
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $archive

$hash = Get-FileHash $archive -Algorithm SHA256
$hash.Hash | Out-File "$archive.sha256" -Encoding ascii

Write-Host "Created $archive"
Write-Host "SHA256 $($hash.Hash)"
