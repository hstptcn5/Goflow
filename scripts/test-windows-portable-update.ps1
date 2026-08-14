[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ApplicationSource,
    [Parameter(Mandatory = $true)][string]$CandidateBundle,
    [Parameter(Mandatory = $true)][string]$Updater
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($env:OS -ne 'Windows_NT') {
    throw 'portable update acceptance must run on Windows'
}

$ApplicationSource = [IO.Path]::GetFullPath($ApplicationSource)
$CandidateBundle = [IO.Path]::GetFullPath($CandidateBundle)
$Updater = [IO.Path]::GetFullPath($Updater)
$TempRoot = Join-Path ([IO.Path]::GetTempPath()) ('goflow-update-acceptance-' + [guid]::NewGuid().ToString('N'))

function New-Fixture {
    param([Parameter(Mandatory = $true)][string]$Name)
    $root = Join-Path $TempRoot $Name
    $app = Join-Path $root 'app'
    $data = Join-Path $root 'external-data'
    New-Item -ItemType Directory -Path $app, $data | Out-Null
    Copy-Item -Path (Join-Path $ApplicationSource '*') -Destination $app -Recurse
    Set-Content -LiteralPath (Join-Path $data 'pilot-sentinel.txt') -Value 'preserve-me' -Encoding ascii
    return @{ Root = $root; App = $app; Data = $data }
}

function Get-FileSHA256 {
    param([Parameter(Mandatory = $true)][string]$Path)
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Assert-Sentinel {
    param([Parameter(Mandatory = $true)][string]$DataDirectory)
    if ((Get-Content -LiteralPath (Join-Path $DataDirectory 'pilot-sentinel.txt') -Raw).Trim() -ne 'preserve-me') {
        throw 'external data sentinel changed during update acceptance'
    }
}

function New-TamperedArchive {
    param([Parameter(Mandatory = $true)][string]$Root)
    $directory = Join-Path $Root 'tampered-candidate'
    Copy-Item -LiteralPath $ApplicationSource -Destination $directory -Recurse
    Add-Content -LiteralPath (Join-Path $directory 'pack\workflows\main.json') -Value ' '
    $archive = Join-Path $Root 'tampered.zip'
    Compress-Archive -Path (Join-Path $directory '*') -DestinationPath $archive
    return $archive
}

function New-UnhealthyArchive {
    param([Parameter(Mandatory = $true)][string]$Root)
    $directory = Join-Path $Root 'unhealthy-candidate'
    Copy-Item -LiteralPath $ApplicationSource -Destination $directory -Recurse
    $manifestPath = Join-Path $directory 'pack\pack.json'
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    $manifest | Add-Member -NotePropertyName plugins -NotePropertyValue @('unsupported.health-fixture')
    [IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json -Depth 20), [Text.UTF8Encoding]::new($false))
    $bytes = [IO.File]::ReadAllBytes($manifestPath)
    $sha = [Security.Cryptography.SHA256]::Create()
    $hash = ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    $infoPath = Join-Path $directory 'PACK_INFO.json'
    $info = Get-Content -LiteralPath $infoPath -Raw | ConvertFrom-Json
    $entry = $info.files | Where-Object path -eq 'pack/pack.json'
    $entry.sha256 = $hash
    $entry.size = $bytes.Length
    [IO.File]::WriteAllText($infoPath, ($info | ConvertTo-Json -Depth 20), [Text.UTF8Encoding]::new($false))
    $archive = Join-Path $Root 'unhealthy.zip'
    Compress-Archive -Path (Join-Path $directory '*') -DestinationPath $archive
    return $archive
}

try {
    New-Item -ItemType Directory -Path $TempRoot | Out-Null

    $success = New-Fixture -Name 'success'
    $output = & $Updater -ApplicationDirectory $success.App -CandidateBundle $CandidateBundle -DataDirectory $success.Data -HealthTimeoutSeconds 20
    if ($output -notcontains 'UNSIGNED-PILOT-BETA' -or $output -notcontains 'status=UPDATED') {
        throw 'valid update did not report the unsigned beta success contract'
    }
    Assert-Sentinel -DataDirectory $success.Data
    & (Join-Path $success.App 'goflow.exe') pack verify $success.App --output json | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw 'updated application failed verification'
    }
    if (@(Get-ChildItem -LiteralPath $success.Root -Directory -Filter 'app.rollback-*').Count -ne 1) {
        throw 'successful update did not retain exactly one application rollback directory'
    }

    $tamper = New-Fixture -Name 'tamper'
    $tamperedArchive = New-TamperedArchive -Root $tamper.Root
    $beforeTamper = Get-FileSHA256 -Path (Join-Path $tamper.App 'pack\workflows\main.json')
    $tamperRejected = $false
    try {
        & $Updater -ApplicationDirectory $tamper.App -CandidateBundle $tamperedArchive -DataDirectory $tamper.Data -HealthTimeoutSeconds 5 | Out-Null
    } catch {
        $tamperRejected = $true
    }
    if (-not $tamperRejected -or $beforeTamper -ne (Get-FileSHA256 -Path (Join-Path $tamper.App 'pack\workflows\main.json'))) {
        throw 'tampered candidate was accepted or mutated the application'
    }
    Assert-Sentinel -DataDirectory $tamper.Data

    $rollback = New-Fixture -Name 'rollback'
    $unhealthyArchive = New-UnhealthyArchive -Root $rollback.Root
    $beforeRollback = Get-FileSHA256 -Path (Join-Path $rollback.App 'goflow.exe')
    $healthRejected = $false
    try {
        & $Updater -ApplicationDirectory $rollback.App -CandidateBundle $unhealthyArchive -DataDirectory $rollback.Data -HealthTimeoutSeconds 5 | Out-Null
    } catch {
        $healthRejected = $true
    }
    if (-not $healthRejected -or $beforeRollback -ne (Get-FileSHA256 -Path (Join-Path $rollback.App 'goflow.exe'))) {
        throw 'unhealthy candidate was accepted or previous application was not restored'
    }
    Assert-Sentinel -DataDirectory $rollback.Data

    $reparse = New-Fixture -Name 'reparse'
    $outside = Join-Path $reparse.Root 'outside-data'
    New-Item -ItemType Directory -Path $outside | Out-Null
    $junction = Join-Path $reparse.Data 'linked-data'
    New-Item -ItemType Junction -Path $junction -Target $outside | Out-Null
    $reparseRejected = $false
    try {
        & $Updater -ApplicationDirectory $reparse.App -CandidateBundle $CandidateBundle -DataDirectory $reparse.Data -HealthTimeoutSeconds 5 | Out-Null
    } catch {
        $reparseRejected = $true
    }
    if (-not $reparseRejected) {
        throw 'data-directory reparse point was accepted'
    }
    Assert-Sentinel -DataDirectory $reparse.Data

    $running = New-Fixture -Name 'running'
    $runningProcess = Start-Process -FilePath (Join-Path $running.App 'goflow.exe') -ArgumentList @(
        'pack', 'run', (Join-Path $running.App 'pack'), '--data-dir', $running.Data,
        '--port', '0', '--no-open'
    ) -WorkingDirectory $running.App -WindowStyle Hidden -PassThru
    try {
        if ($runningProcess.HasExited) {
            throw 'running-instance fixture exited unexpectedly'
        }
        $runningRejected = $false
        try {
            & $Updater -ApplicationDirectory $running.App -CandidateBundle $CandidateBundle -DataDirectory $running.Data -HealthTimeoutSeconds 5 | Out-Null
        } catch {
            $runningRejected = $true
        }
        if (-not $runningRejected) {
            throw 'update accepted an active Goflow instance'
        }
    } finally {
        if (-not $runningProcess.HasExited) {
            Stop-Process -Id $runningProcess.Id -ErrorAction SilentlyContinue
            $runningProcess.WaitForExit(5000) | Out-Null
        }
    }

    Write-Output 'WINDOWS_PORTABLE_UPDATE PASS'
    Write-Output 'valid_candidate=verified-health-checked-backups-retained'
    Write-Output 'tampered_candidate=rejected-before-mutation'
    Write-Output 'unhealthy_candidate=application-and-data-restored'
    Write-Output 'reparse_data=rejected-before-mutation'
    Write-Output 'running_instance=rejected-before-mutation'
} finally {
    $resolvedTemp = [IO.Path]::GetFullPath($TempRoot)
    $systemTemp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if ($resolvedTemp.StartsWith($systemTemp, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedTemp)) {
        for ($attempt = 0; $attempt -lt 10 -and (Test-Path -LiteralPath $resolvedTemp); $attempt++) {
            try {
                Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
            } catch {
                if ($attempt -eq 9) {
                    throw
                }
                Start-Sleep -Milliseconds 250
            }
        }
    }
}
