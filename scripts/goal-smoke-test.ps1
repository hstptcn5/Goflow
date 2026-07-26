param(
  [string]$Binary = ".\goflow.exe",
  [string]$Port = "18080",
  [string]$AdminKey = "goal-admin-key"
)

$ErrorActionPreference = "Stop"
$BaseUrl = "http://127.0.0.1:$Port"
$SmokeDir = Join-Path $env:TEMP ("goflow-goal-smoke-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $SmokeDir | Out-Null

$env:GOFLOW_HOST = "127.0.0.1"
$env:GOFLOW_PORT = $Port
$env:GOFLOW_DB_PATH = Join-Path $SmokeDir "goflow.db"
$env:GOFLOW_MASTER_KEY_FILE = Join-Path $SmokeDir "goflow.master.key"
$env:GOFLOW_API_KEY = $AdminKey
$env:GOFLOW_MCP_ALLOWED_ORIGINS = $BaseUrl
$env:GOFLOW_MAX_CONCURRENT_EXECUTIONS = "10"
$env:GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION = "2"

function Invoke-GoflowApi {
  param([string]$Method, [string]$Path, $Body = $null, [string]$ApiKey = $AdminKey)
  $params = @{
    Method = $Method
    Uri = "$BaseUrl$Path"
    Headers = @{ Authorization = "Bearer $ApiKey" }
  }
  if ($null -ne $Body) {
    $params.ContentType = "application/json"
    $params.Body = ($Body | ConvertTo-Json -Depth 20)
  }
  Invoke-RestMethod @params
}

function New-SmokeWorkflow {
  param([string]$Id, [string]$Name, [int]$Seconds, [string]$ToolName)
  $nodes = ConvertTo-Json -InputObject @(@{
    id = "delay"
    type = "delaySleep"
    name = "Delay"
    params = @{ seconds = [string]$Seconds }
  }) -Compress -Depth 10
  Invoke-GoflowApi -Method Post -Path "/api/v1/workflows" -Body @{
    id = $Id
    name = $Name
    description = "GOAL smoke workflow"
    is_active = $true
    nodes_json = $nodes
    edges_json = "[]"
    slug = $Id
    input_schema_json = "{}"
    output_schema_json = "{}"
    expose_cli = $true
    expose_mcp = $true
    mcp_tool_name = $ToolName
    mcp_description = $Name
    risk_level = "low"
    requires_approval = $false
    max_concurrent_runs = 0
    concurrency_policy = "global"
  }
}

$proc = Start-Process -FilePath (Resolve-Path $Binary) -ArgumentList "serve" -WorkingDirectory (Get-Location) -PassThru -WindowStyle Hidden
try {
  $ready = $false
  for ($i = 0; $i -lt 50; $i++) {
    Start-Sleep -Milliseconds 200
    try {
      & $Binary status --url $BaseUrl --api-key $AdminKey | Out-Null
      $ready = $true
      break
    } catch {
    }
  }
  if (-not $ready) {
    throw "server did not become ready"
  }
  "SMOKE server ready"

  $workflow = New-SmokeWorkflow -Id "wf-goal-smoke" -Name "GOAL Smoke" -Seconds 0 -ToolName "goal_smoke"
  $otherWorkflow = New-SmokeWorkflow -Id "wf-goal-other" -Name "GOAL Other" -Seconds 0 -ToolName "goal_other"
  $cancelWorkflow = New-SmokeWorkflow -Id "wf-goal-cancel" -Name "GOAL Cancel" -Seconds 5 -ToolName "goal_cancel"
  "SMOKE workflows created: $($workflow.id), $($otherWorkflow.id), $($cancelWorkflow.id)"

  & $Binary status --url $BaseUrl --api-key $AdminKey
  & $Binary workflow list --url $BaseUrl --api-key $AdminKey
  & $Binary workflow run $workflow.id --url $BaseUrl --api-key $AdminKey --set source=cli-smoke --wait --timeout 15s

  $token = Invoke-GoflowApi -Method Post -Path "/api/v1/tokens" -Body @{
    name = "goal-scoped"
    scopes = @("workflow:list", "workflow:read", "workflow:run", "execution:read", "execution:cancel")
    allowed_workflows = @($workflow.id)
  }
  $scopedToken = $token.token
  $scopedList = Invoke-GoflowApi -Method Get -Path "/api/v1/workflows" -ApiKey $scopedToken
  if ($scopedList.Count -ne 1 -or $scopedList[0].id -ne $workflow.id) {
    throw "scoped workflow list failed: $($scopedList | ConvertTo-Json -Compress)"
  }
  if (($scopedList[0].PSObject.Properties.Name -contains "nodes_json") -or ($scopedList[0].PSObject.Properties.Name -contains "edges_json")) {
    throw "scoped workflow list leaked graph fields"
  }
  "SMOKE scoped token allowlist passed"

  node scripts/mcp-smoke-test.mjs --binary $Binary --url $BaseUrl --api-key $scopedToken --workflow $workflow.id --input '{\"source\":\"mcp-stdio\"}'
  node scripts/mcp-http-smoke-test.mjs --url "$BaseUrl/mcp" --api-key $scopedToken --origin $BaseUrl --workflow $workflow.id --input '{\"source\":\"mcp-http\"}'

  $cancelRun = Invoke-GoflowApi -Method Post -Path "/api/v1/workflows/$($cancelWorkflow.id)/executions" -Body @{
    mode = "async"
    input = @{ source = "cancel-smoke" }
  }
  & $Binary execution cancel $cancelRun.execution_id --url $BaseUrl --api-key $AdminKey
  Start-Sleep -Milliseconds 500
  $cancelExec = Invoke-GoflowApi -Method Get -Path "/api/v1/executions/$($cancelRun.execution_id)"
  if ($cancelExec.status -ne "CANCELLED") {
    throw "cancellation failed, status=$($cancelExec.status)"
  }
  "SMOKE cancellation passed"

  $idemKey = "smoke-concurrent-idem"
  $jobs = 1..20 | ForEach-Object {
    Start-Job -ScriptBlock {
      param($BaseUrl, $WorkflowID, $Key, $AdminKey)
      Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/workflows/$WorkflowID/executions" -ContentType "application/json" -Headers @{ Authorization = "Bearer $AdminKey" } -Body (@{
        mode = "async"
        idempotency_key = $Key
        input = @{ source = "concurrent-smoke" }
      } | ConvertTo-Json -Depth 5)
    } -ArgumentList $BaseUrl, $workflow.id, $idemKey, $AdminKey
  }
  $responses = $jobs | Wait-Job | Receive-Job
  $jobs | Remove-Job
  $ids = @($responses | ForEach-Object { $_.execution_id } | Select-Object -Unique)
  if ($ids.Count -ne 1) {
    throw "concurrent idempotency returned multiple execution ids: $($ids -join ',')"
  }
  "SMOKE concurrent idempotency passed"

  $audit = Invoke-GoflowApi -Method Get -Path "/api/v1/audit-events?limit=5"
  if ($audit.Count -lt 1) {
    throw "audit smoke returned no events"
  }
  "SMOKE audit passed"
} finally {
  if ($proc -and -not $proc.HasExited) {
    Stop-Process -Id $proc.Id -Force
  }
  Remove-Item Env:\GOFLOW_HOST -ErrorAction SilentlyContinue
  Remove-Item Env:\GOFLOW_PORT -ErrorAction SilentlyContinue
  Remove-Item Env:\GOFLOW_DB_PATH -ErrorAction SilentlyContinue
  Remove-Item Env:\GOFLOW_MASTER_KEY_FILE -ErrorAction SilentlyContinue
  Remove-Item Env:\GOFLOW_API_KEY -ErrorAction SilentlyContinue
  Remove-Item Env:\GOFLOW_MCP_ALLOWED_ORIGINS -ErrorAction SilentlyContinue
  Remove-Item Env:\GOFLOW_MAX_CONCURRENT_EXECUTIONS -ErrorAction SilentlyContinue
  Remove-Item Env:\GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION -ErrorAction SilentlyContinue
}
