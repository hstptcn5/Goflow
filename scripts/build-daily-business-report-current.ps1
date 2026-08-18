[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$OutputDirectory,
    [Parameter(Mandatory = $true)][string]$Commit
)

$ErrorActionPreference = 'Stop'
$pack = Get-Content -Raw (Join-Path $PSScriptRoot '..\examples\packs\daily-business-report\pack.json') | ConvertFrom-Json
$version = [string]$pack.version
if ([string]::IsNullOrWhiteSpace($version)) { throw 'Daily Business Report pack version is missing' }

$sourcePath = Join-Path $PSScriptRoot 'build-daily-business-report.ps1'
$generatedPath = Join-Path $PSScriptRoot '.build-daily-business-report-current.generated.ps1'
$source = Get-Content -Raw -LiteralPath $sourcePath
$source = $source -replace "\$PackVersion = '0\.9\.0'", "`$PackVersion = '$version'"
if ($source -notmatch [regex]::Escape("`$PackVersion = '$version'")) {
    throw 'could not bind the artifact builder to the current pack version'
}
try {
    Set-Content -LiteralPath $generatedPath -Value $source -Encoding utf8
    & $generatedPath -OutputDirectory $OutputDirectory -Commit $Commit
    if ($LASTEXITCODE -ne 0) { throw "Daily Business Report artifact builder exited with code $LASTEXITCODE" }
} finally {
    Remove-Item -LiteralPath $generatedPath -Force -ErrorAction SilentlyContinue
}
