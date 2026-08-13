[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ApplicationDirectory,

    [Parameter(Mandatory = $true)]
    [string]$CandidateBundle,

    [string]$DataDirectory = (Join-Path $env:LOCALAPPDATA 'Goflow\packs\official.dailyops-rest-telegram'),

    [int]$HealthTimeoutSeconds = 30
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$ExpectedPackID = 'official.dailyops-rest-telegram'
$ExpectedTarget = 'windows-amd64'
$ExpectedMarker = 'UNSIGNED-PILOT-BETA'

function Get-FullPath {
    param([Parameter(Mandatory = $true)][string]$Path)
    return [System.IO.Path]::GetFullPath($Path).TrimEnd('\', '/')
}

function Assert-OrdinaryDirectory {
    param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)][string]$Label)
    $item = Get-Item -LiteralPath $Path -Force
    if (-not $item.PSIsContainer -or ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
        throw "$Label must be an ordinary directory, not a link or reparse point"
    }
}

function Assert-NoReparsePoints {
    param([Parameter(Mandatory = $true)][string]$Root, [Parameter(Mandatory = $true)][string]$Label)
    foreach ($item in Get-ChildItem -LiteralPath $Root -Recurse -Force) {
        if ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) {
            throw "$Label contains a link or reparse point"
        }
    }
}

function Invoke-GoflowVerify {
    param([Parameter(Mandatory = $true)][string]$Runtime, [Parameter(Mandatory = $true)][string]$Reference)
    $rawResult = & $Runtime pack verify $Reference --output json
    $exitCode = $LASTEXITCODE
    try {
        $result = $rawResult | ConvertFrom-Json
    } catch {
        throw 'candidate verification did not return valid JSON'
    }
    if ($exitCode -ne 0 -or $result.status -ne 'PASS') {
        throw "candidate verification failed"
    }
}

function Assert-NoRunningInstance {
    param([Parameter(Mandatory = $true)][string]$Runtime)
    $null = Get-FullPath $Runtime
    if (@(Get-Process -Name 'goflow' -ErrorAction SilentlyContinue).Count -gt 0) {
        throw 'Goflow is still running. Stop every Goflow process cleanly with Ctrl+C before updating.'
    }
}

function Wait-GoflowHealth {
    param(
        [Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process,
        [Parameter(Mandatory = $true)][string]$StdoutPath,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        if ($Process.HasExited) {
            throw "updated appliance exited before health verification"
        }
        if (Test-Path -LiteralPath $StdoutPath) {
            [string]$text = Get-Content -LiteralPath $StdoutPath -Raw -ErrorAction SilentlyContinue
            if ($null -eq $text) {
                $text = ''
            }
            $match = [regex]::Match($text, 'URL:\s+(http://127\.0\.0\.1:\d+)/')
            if ($match.Success) {
                try {
                    $response = Invoke-WebRequest -UseBasicParsing -Uri ($match.Groups[1].Value + '/healthz') -TimeoutSec 2
                    if ($response.StatusCode -eq 200) {
                        return
                    }
                } catch {
                    # The listener may be published just before the health route responds.
                }
            }
        }
        Start-Sleep -Milliseconds 200
    }
    throw "updated appliance did not become healthy within $TimeoutSeconds seconds"
}

if ($env:OS -ne 'Windows_NT') {
    throw 'portable update is supported only on Windows'
}
if ($HealthTimeoutSeconds -lt 5 -or $HealthTimeoutSeconds -gt 120) {
    throw 'HealthTimeoutSeconds must be between 5 and 120'
}

$ApplicationDirectory = Get-FullPath $ApplicationDirectory
$CandidateBundle = Get-FullPath $CandidateBundle
$DataDirectory = Get-FullPath $DataDirectory
Assert-OrdinaryDirectory -Path $ApplicationDirectory -Label 'ApplicationDirectory'
Assert-NoReparsePoints -Root $ApplicationDirectory -Label 'ApplicationDirectory'
if (-not [System.IO.Path]::GetExtension($CandidateBundle).Equals('.zip', [System.StringComparison]::OrdinalIgnoreCase)) {
    throw 'CandidateBundle must be a ZIP archive'
}
$CurrentRuntime = Join-Path $ApplicationDirectory 'goflow.exe'
if (-not (Test-Path -LiteralPath $CurrentRuntime -PathType Leaf)) {
    throw 'current application runtime is missing'
}
Assert-NoRunningInstance -Runtime $CurrentRuntime
Invoke-GoflowVerify -Runtime $CurrentRuntime -Reference $CandidateBundle

$stamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ')
$parent = Split-Path -Parent $ApplicationDirectory
$name = Split-Path -Leaf $ApplicationDirectory
$candidateStage = Join-Path $parent ($name + '.candidate-' + $stamp)
$applicationBackup = Join-Path $parent ($name + '.rollback-' + $stamp)
$dataBackup = $DataDirectory + '.rollback-' + $stamp
$failedApplication = Join-Path $parent ($name + '.failed-' + $stamp)
$failedData = $DataDirectory + '.failed-' + $stamp
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('goflow-offline-update-' + [guid]::NewGuid().ToString('N'))
$stdoutPath = Join-Path $tempRoot 'health.stdout.txt'
$stderrPath = Join-Path $tempRoot 'health.stderr.txt'
$activated = $false
$applicationMoved = $false
$dataSnapshotted = $false
$healthProcess = $null

try {
    New-Item -ItemType Directory -Path $tempRoot | Out-Null
    Expand-Archive -LiteralPath $CandidateBundle -DestinationPath $candidateStage
    Assert-OrdinaryDirectory -Path $candidateStage -Label 'extracted candidate'
    $CandidateRuntime = Join-Path $candidateStage 'goflow.exe'
    Invoke-GoflowVerify -Runtime $CurrentRuntime -Reference $candidateStage
    Invoke-GoflowVerify -Runtime $CandidateRuntime -Reference $candidateStage

    $info = Get-Content -LiteralPath (Join-Path $candidateStage 'PACK_INFO.json') -Raw | ConvertFrom-Json
    if ($info.pack_id -ne $ExpectedPackID -or $info.target -ne $ExpectedTarget) {
        throw 'candidate Pack identity or target does not match this appliance'
    }
    if (Test-Path -LiteralPath $DataDirectory) {
        Assert-OrdinaryDirectory -Path $DataDirectory -Label 'DataDirectory'
        Assert-NoReparsePoints -Root $DataDirectory -Label 'DataDirectory'
        Copy-Item -LiteralPath $DataDirectory -Destination $dataBackup -Recurse
        $dataSnapshotted = $true
    }

    Move-Item -LiteralPath $ApplicationDirectory -Destination $applicationBackup
    $applicationMoved = $true
    Move-Item -LiteralPath $candidateStage -Destination $ApplicationDirectory
    $activated = $true

    $updatedRuntime = Join-Path $ApplicationDirectory 'goflow.exe'
    $healthProcess = Start-Process -FilePath $updatedRuntime -ArgumentList @(
        'pack', 'run', (Join-Path $ApplicationDirectory 'pack'),
        '--data-dir', $DataDirectory, '--port', '0', '--no-open'
    ) -WorkingDirectory $ApplicationDirectory -WindowStyle Hidden -PassThru -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
    Wait-GoflowHealth -Process $healthProcess -StdoutPath $stdoutPath -TimeoutSeconds $HealthTimeoutSeconds
    Stop-Process -Id $healthProcess.Id -ErrorAction SilentlyContinue
    $healthProcess.WaitForExit(5000) | Out-Null
    $healthProcess = $null

    Write-Output $ExpectedMarker
    Write-Output "status=UPDATED"
    Write-Output "application_backup=$applicationBackup"
    if ($dataSnapshotted) {
        Write-Output "data_backup=$dataBackup"
    }
    Write-Output 'Restart Goflow and complete any required migration revalidation.'
} catch {
    if ($null -ne $healthProcess -and -not $healthProcess.HasExited) {
        Stop-Process -Id $healthProcess.Id -ErrorAction SilentlyContinue
        $healthProcess.WaitForExit(5000) | Out-Null
    }
    if ($activated) {
        if (Test-Path -LiteralPath $ApplicationDirectory) {
            Move-Item -LiteralPath $ApplicationDirectory -Destination $failedApplication
        }
        Move-Item -LiteralPath $applicationBackup -Destination $ApplicationDirectory
        if ($dataSnapshotted -and (Test-Path -LiteralPath $dataBackup)) {
            if (Test-Path -LiteralPath $DataDirectory) {
                Move-Item -LiteralPath $DataDirectory -Destination $failedData
            }
            Move-Item -LiteralPath $dataBackup -Destination $DataDirectory
        }
    } elseif ($applicationMoved -and (Test-Path -LiteralPath $applicationBackup)) {
        Move-Item -LiteralPath $applicationBackup -Destination $ApplicationDirectory
    }
    if (-not $activated -and (Test-Path -LiteralPath $candidateStage)) {
        Remove-Item -LiteralPath $candidateStage -Recurse -Force
    }
    throw "offline update failed and previous application/data were restored: $($_.Exception.Message)"
} finally {
    if (Test-Path -LiteralPath $tempRoot) {
        for ($attempt = 0; $attempt -lt 10 -and (Test-Path -LiteralPath $tempRoot); $attempt++) {
            try {
                Remove-Item -LiteralPath $tempRoot -Recurse -Force
            } catch {
                if ($attempt -eq 9) {
                    throw
                }
                Start-Sleep -Milliseconds 250
            }
        }
    }
}
