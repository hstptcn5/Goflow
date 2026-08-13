[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$OutputDirectory,

    [Parameter(Mandatory = $true)]
    [string]$Commit
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$ArtifactDirectoryName = 'Goflow-DailyOps-Windows-amd64'
$BundleFileName = 'Goflow-DailyOps-Windows-amd64.zip'
$MarkerFileName = 'UNSIGNED-PILOT-BETA.txt'
$ChecksumFileName = 'SHA256SUMS.txt'
$PackID = 'official.dailyops-rest-telegram'
$PackVersion = '0.3.0'

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Command,

        [Parameter(ValueFromRemainingArguments = $true)]
        [string[]]$Arguments
    )

    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Command exited with code $LASTEXITCODE"
    }
}

function Get-RelativeArtifactPath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Root,

        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    $rootPath = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/') + [System.IO.Path]::DirectorySeparatorChar
    $filePath = [System.IO.Path]::GetFullPath($Path)
    if (-not $filePath.StartsWith($rootPath, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw 'artifact file is outside the expected root'
    }
    return $filePath.Substring($rootPath.Length).Replace('\', '/')
}

function Test-ForbiddenFiles {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Root
    )

    $forbiddenNames = @(
        '.env',
        'goflow.db',
        'goflow.master.key',
        'pack-config.json',
        'pack-credentials.json',
        'pack-setup-state.json',
        'pack-state.json',
        'run-state.json'
    )
    $forbiddenExtensions = @('.db', '.sqlite', '.sqlite3', '.key', '.pem', '.p12')
    $hits = [System.Collections.Generic.List[string]]::new()
    foreach ($file in Get-ChildItem -LiteralPath $Root -Recurse -File) {
        if ($forbiddenNames -contains $file.Name.ToLowerInvariant() -or $forbiddenExtensions -contains $file.Extension.ToLowerInvariant()) {
            $hits.Add((Get-RelativeArtifactPath -Root $Root -Path $file.FullName))
        }
    }
    if ($hits.Count -gt 0) {
        throw "portable artifact contains forbidden runtime state files: $($hits -join ', ')"
    }
}

function Test-ForbiddenContent {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Roots,

        [Parameter(Mandatory = $true)]
        [string[]]$ExactValues
    )

    $fixedValues = @(
        'fake-telegram-value',
        'dailyops-test-master-key',
        'GOFLOW_DAILYOPS_TOKEN_PARTS',
        'GOFLOW_DAILYOPS_SOURCE_URL',
        'dailyops_e2e_bot',
        '@dailyops_e2e'
    )
    $values = @($fixedValues + $ExactValues) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique
    $hits = [System.Collections.Generic.List[string]]::new()
    foreach ($root in $Roots) {
        foreach ($file in Get-ChildItem -LiteralPath $root -Recurse -File) {
            $bytes = [System.IO.File]::ReadAllBytes($file.FullName)
            $content = [System.Text.Encoding]::UTF8.GetString($bytes)
            foreach ($value in $values) {
                $variants = @($value, $value.Replace('\', '/')) | Select-Object -Unique
                foreach ($variant in $variants) {
                    if ($content.IndexOf($variant, [System.StringComparison]::OrdinalIgnoreCase) -ge 0) {
                        $hits.Add("$($file.Name): forbidden value")
                        break
                    }
                }
            }
            if ($file.Extension -notin @('.exe', '.zip')) {
                if ($content -match 'http://127\.0\.0\.1:\d+') {
                    $hits.Add("$($file.Name): concrete loopback mock URL")
                }
                if ($content -match '\b\d{5,}[:|][A-Za-z0-9_-]{16,}\b') {
                    $hits.Add("$($file.Name): token-shaped value")
                }
            }
        }
    }
    if ($hits.Count -gt 0) {
        $uniqueHits = @($hits | Select-Object -Unique | Sort-Object)
        throw "portable artifact content scan failed: $($uniqueHits -join ', ')"
    }
}

if ($env:OS -ne 'Windows_NT') {
    throw 'Windows pilot artifacts must be built natively on Windows'
}

$RepoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$OutputDirectory = [System.IO.Path]::GetFullPath($OutputDirectory)
$TempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("goflow-windows-pilot-build-" + [guid]::NewGuid().ToString('N'))
$TempRoot = [System.IO.Path]::GetFullPath($TempRoot)
$SystemTemp = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())

if (-not $TempRoot.StartsWith($SystemTemp, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw 'refusing to use a temporary directory outside the system temp root'
}
if (Test-Path -LiteralPath $OutputDirectory) {
    if ((Get-ChildItem -LiteralPath $OutputDirectory -Force | Measure-Object).Count -gt 0) {
        throw "output directory must be empty: $OutputDirectory"
    }
} else {
    New-Item -ItemType Directory -Path $OutputDirectory | Out-Null
}
New-Item -ItemType Directory -Path $TempRoot | Out-Null

$previousCGO = $env:CGO_ENABLED
try {
    Push-Location $RepoRoot
    $head = (git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $head -ne $Commit) {
        throw "checked out commit does not match requested workflow commit"
    }
    $goos = (go env GOOS).Trim()
    $goarch = (go env GOARCH).Trim()
    if ($LASTEXITCODE -ne 0 -or $goos -ne 'windows' -or $goarch -ne 'amd64') {
        throw "native target mismatch: expected windows-amd64, observed $goos-$goarch"
    }

    $env:CGO_ENABLED = '0'
    $RuntimePath = Join-Path $TempRoot 'goflow.exe'
    Invoke-Checked -Command go -Arguments @('build', '-trimpath', '-ldflags=-s -w', '-o', $RuntimePath, 'main.go', 'static_embed.go')

    $BuildOne = Join-Path $TempRoot 'build-one'
    $BuildTwo = Join-Path $TempRoot 'build-two'
    $Extracted = Join-Path $TempRoot 'extracted'
    New-Item -ItemType Directory -Path $BuildOne, $BuildTwo, $Extracted | Out-Null
    Invoke-Checked -Command $RuntimePath -Arguments @('pack', 'build', 'examples/packs/dailyops-rest-telegram', '--output', $BuildOne)
    Invoke-Checked -Command $RuntimePath -Arguments @('pack', 'build', 'examples/packs/dailyops-rest-telegram', '--output', $BuildTwo)

    $ArchiveOne = @(Get-ChildItem -LiteralPath $BuildOne -Filter '*.zip' -File)
    $ArchiveTwo = @(Get-ChildItem -LiteralPath $BuildTwo -Filter '*.zip' -File)
    if ($ArchiveOne.Count -ne 1 -or $ArchiveTwo.Count -ne 1) {
        throw 'native pack build did not produce exactly one bundle per deterministic build'
    }
    $BundleDigest = (Get-FileHash -Algorithm SHA256 -LiteralPath $ArchiveOne[0].FullName).Hash.ToLowerInvariant()
    $SecondDigest = (Get-FileHash -Algorithm SHA256 -LiteralPath $ArchiveTwo[0].FullName).Hash.ToLowerInvariant()
    if ($BundleDigest -ne $SecondDigest) {
        throw 'native Windows DailyOps bundle build is not deterministic'
    }

    Invoke-Checked -Command $RuntimePath -Arguments @('pack', 'verify', $ArchiveOne[0].FullName, '--output', 'json') | Out-Null
    Expand-Archive -LiteralPath $ArchiveOne[0].FullName -DestinationPath $Extracted
    Invoke-Checked -Command $RuntimePath -Arguments @('pack', 'verify', $Extracted, '--output', 'json') | Out-Null
    Invoke-Checked -Command go -Arguments @('run', './internal/testharness/windowspilot', '--app-dir', $Extracted)
    & (Join-Path $RepoRoot 'scripts\test-windows-portable-update.ps1') `
        -ApplicationSource $Extracted `
        -CandidateBundle $ArchiveOne[0].FullName `
        -Updater (Join-Path $RepoRoot 'scripts\update-windows-portable.ps1')

    $PortableDirectory = Join-Path $OutputDirectory $ArtifactDirectoryName
    New-Item -ItemType Directory -Path $PortableDirectory | Out-Null
    Copy-Item -Path (Join-Path $Extracted '*') -Destination $PortableDirectory -Recurse
    $PublishedBundle = Join-Path $OutputDirectory $BundleFileName
    Copy-Item -LiteralPath $ArchiveOne[0].FullName -Destination $PublishedBundle
    Copy-Item -LiteralPath (Join-Path $RepoRoot 'scripts\update-windows-portable.ps1') -Destination (Join-Path $OutputDirectory 'Update-Goflow.ps1')
    Copy-Item -LiteralPath (Join-Path $RepoRoot 'docs\WINDOWS_PILOT_GUIDE.md') -Destination (Join-Path $OutputDirectory 'WINDOWS_PILOT_GUIDE.md')

    $MarkerPath = Join-Path $OutputDirectory $MarkerFileName
    @(
        'UNSIGNED-PILOT-BETA'
        "commit=$Commit"
        'target=windows-amd64'
        "pack_id=$PackID"
        "pack_version=$PackVersion"
        "bundle=$BundleFileName"
        "built_at=$([DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ'))"
    ) | Set-Content -LiteralPath $MarkerPath -Encoding ascii

    Test-ForbiddenFiles -Root $OutputDirectory
    $exactPaths = @(@(
        $RepoRoot,
        $TempRoot,
        $env:GITHUB_WORKSPACE,
        $env:RUNNER_WORKSPACE,
        $env:RUNNER_TEMP
    ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    Test-ForbiddenContent -Roots @($OutputDirectory) -ExactValues $exactPaths

    $ChecksumPath = Join-Path $OutputDirectory $ChecksumFileName
    $checksumEntries = @(
        Get-ChildItem -LiteralPath $OutputDirectory -Recurse -File |
            Where-Object { $_.FullName -ne $ChecksumPath } |
            ForEach-Object {
                $digest = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant()
                $relative = Get-RelativeArtifactPath -Root $OutputDirectory -Path $_.FullName
                "$digest  $relative"
            } |
            Sort-Object
    )
    $checksumEntries | Set-Content -LiteralPath $ChecksumPath -Encoding ascii

    $ArtifactDigest = (Get-FileHash -Algorithm SHA256 -LiteralPath $ChecksumPath).Hash.ToLowerInvariant()
    Write-Output 'WINDOWS_PILOT_ARTIFACT PASS'
    Write-Output "artifact_name=UNSIGNED-PILOT-BETA-goflow-dailyops-windows-amd64"
    Write-Output "bundle_file=$BundleFileName"
    Write-Output "bundle_sha256=$BundleDigest"
    Write-Output "checksums_sha256=$ArtifactDigest"
} finally {
    Pop-Location
    $env:CGO_ENABLED = $previousCGO
    if (Test-Path -LiteralPath $TempRoot) {
        $resolvedTemp = [System.IO.Path]::GetFullPath($TempRoot)
        if ($resolvedTemp.StartsWith($SystemTemp, [System.StringComparison]::OrdinalIgnoreCase)) {
            Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
        }
    }
}
