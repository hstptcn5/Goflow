param(
  [string]$Binary = "",
  [string]$Port = "18110",
  [string]$AdminKey = "roadmap-smoke-admin-key",
  [switch]$RequirePython,
  [switch]$KeepTemp
)

$ErrorActionPreference = "Stop"
$BaseUrl = "http://127.0.0.1:$Port"
$SmokeDir = Join-Path $env:TEMP ("goflow-roadmap-smoke-" + [guid]::NewGuid().ToString("N"))
$script:Proc = $null
$script:Passed = 0
$script:Failed = 0
$script:Skipped = 0
$script:BuiltBinary = $false

New-Item -ItemType Directory -Path $SmokeDir | Out-Null
$FileRoot = Join-Path $SmokeDir "files"
$CustomNodeDir = Join-Path $SmokeDir "custom-nodes"
New-Item -ItemType Directory -Path $FileRoot | Out-Null
New-Item -ItemType Directory -Path $CustomNodeDir | Out-Null

$env:GOFLOW_HOST = "127.0.0.1"
$env:GOFLOW_PORT = $Port
$env:GOFLOW_DB_PATH = Join-Path $SmokeDir "goflow.db"
$env:GOFLOW_MASTER_KEY_FILE = Join-Path $SmokeDir "goflow.master.key"
$env:GOFLOW_API_KEY = $AdminKey
$env:GOFLOW_FILE_ALLOWED_ROOTS = $FileRoot
$env:GOFLOW_FILE_STORE_DIR = Join-Path $SmokeDir "file-store"
$env:GOFLOW_CUSTOM_NODE_DIR = $CustomNodeDir
$env:GOFLOW_MAX_CONCURRENT_EXECUTIONS = "8"
$env:GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION = "4"

function Write-Pass {
  param([string]$Name)
  $script:Passed++
  Write-Host "[PASS] $Name"
}

function Write-Fail {
  param([string]$Name, [string]$Message)
  $script:Failed++
  Write-Host "[FAIL] $Name -- $Message"
}

function Write-Skip {
  param([string]$Name, [string]$Reason)
  $script:Skipped++
  Write-Host "[SKIP] $Name -- $Reason"
}

function Invoke-SmokeCase {
  param([string]$Name, [scriptblock]$Body)
  try {
    & $Body
    Write-Pass $Name
  } catch {
    Write-Fail $Name $_.Exception.Message
  }
}

function Invoke-GoflowApi {
  param([string]$Method, [string]$Path, $Body = $null)
  $params = @{
    Method = $Method
    Uri = "$BaseUrl$Path"
    Headers = @{ Authorization = "Bearer $AdminKey" }
  }
  if ($null -ne $Body) {
    $params.ContentType = "application/json"
    $params.Body = ($Body | ConvertTo-Json -Depth 40)
  }
  try {
    Invoke-RestMethod @params
  } catch {
    $detail = $_.ErrorDetails.Message
    if ([string]::IsNullOrWhiteSpace($detail)) {
      $detail = $_.Exception.Message
    }
    throw "$Method $Path failed: $detail"
  }
}

function New-QAWorkflow {
  param([string]$Id, [string]$Name, [array]$Nodes, [array]$Edges = @())
  $nodesJson = ConvertTo-Json -InputObject @($Nodes) -Compress -Depth 40
  $edgesJson = ConvertTo-Json -InputObject @($Edges) -Compress -Depth 40
  Invoke-GoflowApi -Method Post -Path "/api/v1/workflows" -Body @{
    id = $Id
    name = $Name
    description = "Technical roadmap smoke fixture"
    is_active = $true
    nodes_json = $nodesJson
    edges_json = $edgesJson
    slug = $Id
    input_schema_json = "{}"
    output_schema_json = "{}"
    expose_cli = $false
    expose_mcp = $false
    risk_level = "low"
    requires_approval = $false
    max_concurrent_runs = 0
    concurrency_policy = "global"
  }
}

function Wait-QAExecution {
  param([string]$ExecutionId, [int]$TimeoutSeconds = 20)
  $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
  while ([DateTime]::UtcNow -lt $deadline) {
    Start-Sleep -Milliseconds 200
    $execution = Invoke-GoflowApi -Method Get -Path "/api/v1/executions/$ExecutionId"
    if (@("SUCCESS", "FAILED", "CANCELLED", "INTERRUPTED", "REJECTED") -contains $execution.status) {
      return $execution
    }
  }
  throw "execution $ExecutionId did not finish within $TimeoutSeconds seconds"
}

function Invoke-QAWorkflow {
  param([string]$WorkflowId, $Input = @{}, [int]$TimeoutSeconds = 20)
  $run = Invoke-GoflowApi -Method Post -Path "/api/v1/workflows/$WorkflowId/executions" -Body @{
    mode = "async"
    input = $Input
  }
  Wait-QAExecution -ExecutionId $run.execution_id -TimeoutSeconds $TimeoutSeconds
}

function Get-NodeLog {
  param($Execution, [string]$NodeId)
  $log = @($Execution.node_logs | Where-Object { $_.node_id -eq $NodeId } | Select-Object -Last 1)
  if ($log.Count -eq 0) {
    throw "execution $($Execution.id) has no node log for $NodeId"
  }
  return $log[0]
}

function Assert-Equal {
  param($Actual, $Expected, [string]$Message)
  if ($Actual -ne $Expected) {
    throw "$Message (expected=$Expected actual=$Actual)"
  }
}

function Assert-True {
  param([bool]$Condition, [string]$Message)
  if (-not $Condition) {
    throw $Message
  }
}

function Start-Goflow {
  if ($script:Proc -and -not $script:Proc.HasExited) {
    return
  }
  $script:Proc = Start-Process -FilePath $script:BinaryPath -ArgumentList "serve" -WorkingDirectory (Get-Location) -PassThru -WindowStyle Hidden
  $ready = $false
  for ($i = 0; $i -lt 80; $i++) {
    Start-Sleep -Milliseconds 200
    try {
      & $script:BinaryPath status --url $BaseUrl --api-key $AdminKey *> $null
      if ($LASTEXITCODE -eq 0) {
        $ready = $true
        break
      }
    } catch {
    }
  }
  if (-not $ready) {
    throw "Goflow server did not become ready on $BaseUrl"
  }
}

function Stop-Goflow {
  if ($script:Proc -and -not $script:Proc.HasExited) {
    Stop-Process -Id $script:Proc.Id -Force
    $script:Proc.WaitForExit()
  }
  $script:Proc = $null
}

try {
  if ([string]::IsNullOrWhiteSpace($Binary)) {
    $script:BinaryPath = Join-Path $SmokeDir "goflow-roadmap-smoke.exe"
    Write-Host "Building Goflow smoke binary..."
    & go build -o $script:BinaryPath main.go static_embed.go
    if ($LASTEXITCODE -ne 0) {
      throw "go build failed"
    }
    $script:BuiltBinary = $true
  } else {
    $script:BinaryPath = (Resolve-Path $Binary).Path
  }

  Start-Goflow
  Write-Host "GOFLOW TECHNICAL ROADMAP SMOKE"
  Write-Host "Server: $BaseUrl"
  Write-Host "Temp:   $SmokeDir"
  Write-Host ""

  Invoke-SmokeCase "Rich node definitions" {
    $definitions = @(Invoke-GoflowApi -Method Get -Path "/api/v1/nodes/definitions")
    foreach ($type in @("conditionIf", "switch", "workflowState", "pythonCode", "localFile", "fileTrigger", "tableFile", "httpRequest")) {
      Assert-True (@($definitions | Where-Object { $_.type -eq $type }).Count -eq 1) "missing node definition $type"
    }
    $python = $definitions | Where-Object { $_.type -eq "pythonCode" } | Select-Object -First 1
    $pythonEnvironment = $python.params | Where-Object { $_.name -eq "environment" } | Select-Object -First 1
    Assert-Equal $pythonEnvironment.type "select" "Python environment is not a selector"
    $http = $definitions | Where-Object { $_.type -eq "httpRequest" } | Select-Object -First 1
    $authHeader = $http.params | Where-Object { $_.name -eq "auth_header" } | Select-Object -First 1
    Assert-True ($null -ne $authHeader.visible_when) "HTTP auth_header is missing visible_when metadata"
    $onError = $http.params | Where-Object { $_.name -eq "on_error" } | Select-Object -First 1
    Assert-True ($null -ne $onError) "common On Error parameter is missing"
  }

  Invoke-SmokeCase "Typed IF routing" {
    $wf = New-QAWorkflow -Id "wf-smoke-if" -Name "Smoke Typed IF" -Nodes @(
      @{ id = "seed"; type = "jsCodeRunner"; name = "Seed"; params = @{ code = 'return {score: 85, active: true};' } },
      @{ id = "if"; type = "conditionIf"; name = "IF"; params = @{ field = "{{seed.score}}"; operator = "greater_than"; value = 80 } },
      @{ id = "yes"; type = "jsCodeRunner"; name = "Yes"; params = @{ code = 'return {branch: "true"};' } },
      @{ id = "no"; type = "jsCodeRunner"; name = "No"; params = @{ code = 'return {branch: "false"};' } }
    ) -Edges @(
      @{ id = "e1"; source = "seed"; target = "if" },
      @{ id = "e2"; source = "if"; sourceHandle = "true"; target = "yes" },
      @{ id = "e3"; source = "if"; sourceHandle = "false"; target = "no" }
    )
    $exec = Invoke-QAWorkflow $wf.id
    Assert-Equal $exec.status "SUCCESS" "typed IF workflow failed"
    Assert-Equal (Get-NodeLog $exec "if").output.result $true "typed numeric IF did not evaluate true"
    Assert-Equal (Get-NodeLog $exec "yes").status "SUCCESS" "true branch did not run"
    Assert-Equal (Get-NodeLog $exec "no").status "SKIPPED" "false branch unexpectedly ran"
  }

  Invoke-SmokeCase "Switch routing" {
    $cases = '[{"handle":"gold","operator":"equals","value":"gold"},{"handle":"silver","operator":"equals","value":"silver"}]'
    $wf = New-QAWorkflow -Id "wf-smoke-switch" -Name "Smoke Switch" -Nodes @(
      @{ id = "seed"; type = "jsCodeRunner"; name = "Seed"; params = @{ code = 'return {tier: "gold"};' } },
      @{ id = "switch"; type = "switch"; name = "Switch"; params = @{ value = "{{seed.tier}}"; cases_json = $cases } },
      @{ id = "gold"; type = "jsCodeRunner"; name = "Gold"; params = @{ code = 'return "gold";' } },
      @{ id = "silver"; type = "jsCodeRunner"; name = "Silver"; params = @{ code = 'return "silver";' } },
      @{ id = "default"; type = "jsCodeRunner"; name = "Default"; params = @{ code = 'return "default";' } }
    ) -Edges @(
      @{ id = "e1"; source = "seed"; target = "switch" },
      @{ id = "e2"; source = "switch"; sourceHandle = "gold"; target = "gold" },
      @{ id = "e3"; source = "switch"; sourceHandle = "silver"; target = "silver" },
      @{ id = "e4"; source = "switch"; sourceHandle = "default"; target = "default" }
    )
    $exec = Invoke-QAWorkflow $wf.id
    Assert-Equal $exec.status "SUCCESS" "Switch workflow failed"
    Assert-Equal (Get-NodeLog $exec "switch").output.matched_handle "gold" "Switch selected wrong handle"
    Assert-Equal (Get-NodeLog $exec "gold").status "SUCCESS" "gold branch did not run"
    Assert-Equal (Get-NodeLog $exec "silver").status "SKIPPED" "silver branch unexpectedly ran"
    Assert-Equal (Get-NodeLog $exec "default").status "SKIPPED" "default branch unexpectedly ran"
  }

  Invoke-SmokeCase "Handled error output routing" {
    $wf = New-QAWorkflow -Id "wf-smoke-error" -Name "Smoke Error Routing" -Nodes @(
      @{ id = "bad"; type = "jsCodeRunner"; name = "Bad"; params = @{ code = 'throw new Error("roadmap-smoke-boom");'; on_error = "Continue via error output" } },
      @{ id = "normal"; type = "jsCodeRunner"; name = "Normal"; params = @{ code = 'return "normal";' } },
      @{ id = "handled"; type = "jsCodeRunner"; name = "Handled"; params = @{ code = 'return "handled";' } }
    ) -Edges @(
      @{ id = "e1"; source = "bad"; target = "normal" },
      @{ id = "e2"; source = "bad"; sourceHandle = "error"; target = "handled" }
    )
    $exec = Invoke-QAWorkflow $wf.id
    Assert-Equal $exec.status "SUCCESS" "handled node failure incorrectly failed the workflow"
    Assert-Equal (Get-NodeLog $exec "bad").status "FAILED" "failed node did not remain FAILED"
    Assert-Equal (Get-NodeLog $exec "normal").status "SKIPPED" "normal output ran after handled failure"
    Assert-Equal (Get-NodeLog $exec "handled").status "SUCCESS" "error output did not run"
  }

  Invoke-SmokeCase "HTTP request and cURL secret split" {
    $wf = New-QAWorkflow -Id "wf-smoke-http" -Name "Smoke HTTP" -Nodes @(
      @{ id = "http"; type = "httpRequest"; name = "HTTP"; params = @{ method = "GET"; url = "$BaseUrl/healthz"; query_params = @{ smoke = "yes" }; response_mode = "json" } }
    )
    $exec = Invoke-QAWorkflow $wf.id
    Assert-Equal $exec.status "SUCCESS" "local HTTP request failed"
    $output = (Get-NodeLog $exec "http").output
    Assert-Equal $output.status_code 200 "HTTP status code mismatch"
    Assert-Equal $output.data.status "ok" "HTTP JSON response mismatch"

    $secret = "ROADMAP-SMOKE-SECRET-123"
    $import = Invoke-GoflowApi -Method Post -Path "/api/v1/http/import-curl" -Body @{
      command = "curl https://example.invalid/api -H `"Authorization: Bearer $secret`""
    }
    Assert-Equal $import.credential_secret $secret "cURL importer did not isolate bearer secret"
    $paramsJson = $import.params | ConvertTo-Json -Compress -Depth 20
    Assert-True (-not $paramsJson.Contains($secret)) "cURL importer leaked secret into workflow params"
  }

  $stateWorkflow = $null
  Invoke-SmokeCase "Workflow State increment" {
    $stateWorkflow = New-QAWorkflow -Id "wf-smoke-state" -Name "Smoke State" -Nodes @(
      @{ id = "state"; type = "workflowState"; name = "State"; params = @{ operation = "INCREMENT"; scope = "Global"; key = "roadmap_counter"; delta = 1 } }
    )
    $first = Invoke-QAWorkflow $stateWorkflow.id
    $second = Invoke-QAWorkflow $stateWorkflow.id
    Assert-Equal (Get-NodeLog $first "state").output.value 1 "first increment mismatch"
    Assert-Equal (Get-NodeLog $second "state").output.value 2 "second increment mismatch"
  }

  $pythonCommand = $null
  foreach ($candidate in @("python3", "python", "py")) {
    if (Get-Command $candidate -ErrorAction SilentlyContinue) {
      $pythonCommand = $candidate
      break
    }
  }
  if ($null -eq $pythonCommand) {
    if ($RequirePython) {
      Write-Fail "Python execution" "no Python interpreter found"
      Write-Fail "Python timeout" "no Python interpreter found"
    } else {
      Write-Skip "Python execution" "no external CPython interpreter found"
      Write-Skip "Python timeout" "no external CPython interpreter found"
    }
  } else {
    Invoke-SmokeCase "Python execution" {
      $wf = New-QAWorkflow -Id "wf-smoke-python" -Name "Smoke Python" -Nodes @(
        @{ id = "python"; type = "pythonCode"; name = "Python"; params = @{ input = @{ numbers = @(1, 2, 3, 4, 5) }; code = 'output = {"sum": sum(input["numbers"]), "count": len(input["numbers"])}'; timeout = 5 } }
      )
      $exec = Invoke-QAWorkflow $wf.id
      Assert-Equal $exec.status "SUCCESS" "Python workflow failed"
      Assert-Equal (Get-NodeLog $exec "python").output.sum 15 "Python sum mismatch"
      Assert-Equal (Get-NodeLog $exec "python").output.count 5 "Python count mismatch"
    }

    Invoke-SmokeCase "Python timeout" {
      $wf = New-QAWorkflow -Id "wf-smoke-python-timeout" -Name "Smoke Python Timeout" -Nodes @(
        @{ id = "python"; type = "pythonCode"; name = "Python Timeout"; params = @{ code = "import time`ntime.sleep(3)`noutput = {'done': True}"; timeout = 1 } }
      )
      $exec = Invoke-QAWorkflow $wf.id -TimeoutSeconds 10
      Assert-Equal $exec.status "FAILED" "Python timeout workflow did not fail"
      $log = Get-NodeLog $exec "python"
      Assert-True ($log.error -match "timed out") "Python timeout error was not reported"
    }
  }

  $roundtripPath = Join-Path $FileRoot "roundtrip.xlsx"
  Invoke-SmokeCase "FileRef XLSX round-trip" {
    $table = @{
      columns = @("sku", "name", "qty")
      rows = @(
        @{ sku = "A001"; name = "Product A"; qty = 10 },
        @{ sku = "B002"; name = "Product B"; qty = 20 }
      )
    }
    $wf = New-QAWorkflow -Id "wf-smoke-file-roundtrip" -Name "Smoke File Roundtrip" -Nodes @(
      @{ id = "make"; type = "tableFile"; name = "Make XLSX"; params = @{ operation = "WRITE"; format = "XLSX"; table = $table; name = "roundtrip.xlsx" } },
      @{ id = "write"; type = "localFile"; name = "Write"; params = @{ operation = "WRITE"; path = $roundtripPath; file_ref = "{{make}}"; create_directories = $true } },
      @{ id = "read"; type = "localFile"; name = "Read"; params = @{ operation = "READ"; path = $roundtripPath } },
      @{ id = "parse"; type = "tableFile"; name = "Parse"; params = @{ operation = "READ"; format = "AUTO"; file_ref = "{{read}}" } }
    ) -Edges @(
      @{ id = "e1"; source = "make"; target = "write" },
      @{ id = "e2"; source = "write"; target = "read" },
      @{ id = "e3"; source = "read"; target = "parse" }
    )
    $exec = Invoke-QAWorkflow $wf.id
    Assert-Equal $exec.status "SUCCESS" "FileRef/XLSX round-trip workflow failed"
    Assert-True (Test-Path $roundtripPath) "Local File did not write XLSX"
    $output = (Get-NodeLog $exec "parse").output
    Assert-Equal $output.columns.Count 3 "XLSX column count mismatch"
    Assert-Equal $output.rows.Count 2 "XLSX row count mismatch"
    Assert-Equal $output.rows[0].sku "A001" "XLSX first row mismatch"
    Assert-Equal $output.rows[1].qty "20" "XLSX second row mismatch"
  }

  Invoke-SmokeCase "Local File root protection" {
    $outsidePath = Join-Path $env:TEMP ("goflow-roadmap-outside-" + [guid]::NewGuid().ToString("N") + ".txt")
    try {
      Set-Content -Path $outsidePath -Value "outside" -NoNewline
      $wf = New-QAWorkflow -Id "wf-smoke-file-root" -Name "Smoke File Root" -Nodes @(
        @{ id = "read"; type = "localFile"; name = "Read Outside"; params = @{ operation = "READ"; path = $outsidePath } }
      )
      $exec = Invoke-QAWorkflow $wf.id
      Assert-Equal $exec.status "FAILED" "Local File accepted a path outside configured roots"
      Assert-True ((Get-NodeLog $exec "read").error -match "outside GOFLOW_FILE_ALLOWED_ROOTS") "Local File returned an unexpected root-protection error"
    } finally {
      Remove-Item $outsidePath -Force -ErrorAction SilentlyContinue
    }
  }

  $watchDir = Join-Path $FileRoot "incoming"
  New-Item -ItemType Directory -Path $watchDir | Out-Null
  $watchWorkflow = $null
  Invoke-SmokeCase "File Watch created event" {
    $watchWorkflow = New-QAWorkflow -Id "wf-smoke-file-watch" -Name "Smoke File Watch" -Nodes @(
      @{ id = "watch"; type = "fileTrigger"; name = "Watch"; params = @{ path = $watchDir; pattern = "*.txt"; emit_existing = $false } }
    )
    $initial = Invoke-QAWorkflow $watchWorkflow.id
    Assert-Equal (Get-NodeLog $initial "watch").output.count 0 "initial File Watch should only establish snapshot"
    Set-Content -Path (Join-Path $watchDir "created.txt") -Value "created" -NoNewline
    $changed = Invoke-QAWorkflow $watchWorkflow.id
    $output = (Get-NodeLog $changed "watch").output
    Assert-Equal $output.count 1 "File Watch did not emit one CREATED event"
    Assert-Equal $output.events[0].event "CREATED" "File Watch emitted wrong event type"
    Assert-Equal $output.events[0].name "created.txt" "File Watch emitted wrong file"
  }

  $customWorkflow = $null
  Invoke-SmokeCase "Promote reusable JS node" {
    $promoted = Invoke-GoflowApi -Method Post -Path "/api/v1/custom-nodes/promote" -Body @{
      schema_version = 1
      type = "user.smokeDouble"
      name = "Smoke Double"
      version = "1.0.0"
      runtime = "js"
      code = "return input * 2;"
      inputs = @(
        @{ name = "input"; label = "Input"; type = "number"; required = $true }
      )
      outputs = @()
    }
    Assert-Equal $promoted.status "registered" "reusable node was not registered"
    $customWorkflow = New-QAWorkflow -Id "wf-smoke-custom" -Name "Smoke Custom" -Nodes @(
      @{ id = "custom"; type = "user.smokeDouble"; name = "Custom"; params = @{ input = 21 } }
    )
    $exec = Invoke-QAWorkflow $customWorkflow.id
    Assert-Equal $exec.status "SUCCESS" "promoted reusable node failed"
    Assert-Equal (Get-NodeLog $exec "custom").output 42 "promoted reusable node returned wrong output"
  }

  Invoke-SmokeCase "Persistence after restart" {
    Stop-Goflow
    Start-Goflow

    $getWorkflow = New-QAWorkflow -Id "wf-smoke-state-get" -Name "Smoke State Get" -Nodes @(
      @{ id = "state"; type = "workflowState"; name = "State Get"; params = @{ operation = "GET"; scope = "Global"; key = "roadmap_counter" } }
    )
    $stateExec = Invoke-QAWorkflow $getWorkflow.id
    $stateOutput = (Get-NodeLog $stateExec "state").output
    Assert-Equal $stateOutput.found $true "Workflow State disappeared after restart"
    Assert-Equal $stateOutput.value 2 "Workflow State value changed after restart"

    $watchExec = Invoke-QAWorkflow $watchWorkflow.id
    Assert-Equal (Get-NodeLog $watchExec "watch").output.count 0 "File Watch snapshot did not survive restart"

    $customExec = Invoke-QAWorkflow $customWorkflow.id
    Assert-Equal $customExec.status "SUCCESS" "promoted reusable node was not rediscovered after restart"
    Assert-Equal (Get-NodeLog $customExec "custom").output 42 "rediscovered reusable node returned wrong output"
  }

  Invoke-SmokeCase "Pack validate smoke" {
    & $script:BinaryPath pack validate examples/packs/hello-webhook *> $null
    if ($LASTEXITCODE -ne 0) {
      throw "pack validate failed for hello-webhook"
    }
  }

  Write-Host ""
  Write-Host "GOFLOW ROADMAP SMOKE SUMMARY"
  Write-Host "Passed:  $script:Passed"
  Write-Host "Failed:  $script:Failed"
  Write-Host "Skipped: $script:Skipped"
  Write-Host ""
  Write-Host "External-service checkpoints (PostgreSQL/MySQL, Google Sheets/Drive, SMTP and AI providers) are intentionally not called by this local smoke; their deterministic contracts remain covered by the repository test suite."

  if ($script:Failed -gt 0) {
    throw "$script:Failed technical roadmap smoke case(s) failed"
  }
} finally {
  Stop-Goflow
  foreach ($name in @(
    "GOFLOW_HOST",
    "GOFLOW_PORT",
    "GOFLOW_DB_PATH",
    "GOFLOW_MASTER_KEY_FILE",
    "GOFLOW_API_KEY",
    "GOFLOW_FILE_ALLOWED_ROOTS",
    "GOFLOW_FILE_STORE_DIR",
    "GOFLOW_CUSTOM_NODE_DIR",
    "GOFLOW_MAX_CONCURRENT_EXECUTIONS",
    "GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION"
  )) {
    Remove-Item "Env:\$name" -ErrorAction SilentlyContinue
  }
  if (-not $KeepTemp) {
    Remove-Item $SmokeDir -Recurse -Force -ErrorAction SilentlyContinue
  } else {
    Write-Host "Smoke temp retained at $SmokeDir"
  }
}
