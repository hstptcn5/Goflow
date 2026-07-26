#!/usr/bin/env node

const args = parseArgs(process.argv.slice(2));
const url = trimRight(args.url || process.env.GOFLOW_MCP_URL || "http://127.0.0.1:8080/mcp", "/");
const apiKey = args.apiKey || process.env.GOFLOW_API_KEY || "";
const origin = args.origin || "";
const workflow = args.workflow || "";
const input = parseInput(args.input);
const idempotencyKey = args.idempotencyKey || `mcp-http-smoke-${Date.now()}`;
const dynamicTool = args.dynamicTool || "";
const expectTool = args.expectTool || dynamicTool;

await request(1, "initialize", {
  protocolVersion: "2025-06-18",
  capabilities: {},
  clientInfo: { name: "goflow-mcp-http-smoke-test", version: "0.0.0" },
});
const tools = await request(2, "tools/list", {});
const toolNames = (tools.result?.tools || []).map((tool) => tool.name);
assertIncludes(toolNames, "goflow_list_workflows");
assertIncludes(toolNames, "goflow_run_workflow");
assertIncludes(toolNames, "goflow_get_execution");
assertIncludes(toolNames, "goflow_reload_tools");
if (expectTool) {
  assertIncludes(toolNames, expectTool);
}
console.log(`MCP HTTP tools/list passed (${toolNames.length} tools)`);

const workflowList = await callTool(3, "goflow_list_workflows", {});
const workflowCount = workflowList.result?.structuredContent?.count ?? 0;
console.log(`MCP HTTP goflow_list_workflows passed (${workflowCount} workflows)`);

if (dynamicTool) {
  const run = await callTool(4, dynamicTool, input);
  assertToolSuccess(run, dynamicTool);
  const output = run.result?.structuredContent;
  if (!output?.execution_id) {
    throw new Error(`Dynamic tool response has no execution_id: ${JSON.stringify(run)}`);
  }
  console.log(`MCP HTTP ${dynamicTool} passed (${output.execution_id}, ${output.status})`);
} else if (workflow) {
  const run = await callTool(4, "goflow_run_workflow", {
    workflow,
    input,
    idempotency_key: idempotencyKey,
  });
  assertToolSuccess(run, "goflow_run_workflow");
  const output = run.result?.structuredContent;
  if (!output?.execution_id) {
    throw new Error(`goflow_run_workflow response has no execution_id: ${JSON.stringify(run)}`);
  }
  console.log(`MCP HTTP goflow_run_workflow passed (${output.execution_id}, ${output.status})`);

  const execution = await callTool(5, "goflow_get_execution", {
    execution_id: output.execution_id,
  });
  assertToolSuccess(execution, "goflow_get_execution");
  const executionOutput = execution.result?.structuredContent;
  if (!executionOutput?.execution?.status) {
    throw new Error(`goflow_get_execution response has no execution status: ${JSON.stringify(execution)}`);
  }
  assertNoRawInputLeak(executionOutput);
  console.log(`MCP HTTP goflow_get_execution passed (${executionOutput.execution.status})`);
}

async function callTool(id, name, toolArgs) {
  return request(id, "tools/call", {
    name,
    arguments: toolArgs,
  });
}

async function request(id, method, params) {
  const headers = {
    "Content-Type": "application/json",
    Accept: "application/json, text/event-stream",
  };
  if (apiKey) {
    headers.Authorization = `Bearer ${apiKey}`;
  }
  if (origin) {
    headers.Origin = origin;
  }
  const response = await fetch(url, {
    method: "POST",
    headers,
    body: JSON.stringify({ jsonrpc: "2.0", id, method, params }),
  });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`MCP HTTP ${method} failed (${response.status}): ${text}`);
  }
  const payload = parseJSONOrSSE(text);
  if (payload.error) {
    throw new Error(`MCP HTTP error for ${method}: ${JSON.stringify(payload.error)}`);
  }
  return payload;
}

function parseInput(raw) {
  if (!raw) {
    return {};
  }
  try {
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new Error("input must be a JSON object");
    }
    return parsed;
  } catch (error) {
    throw new Error(`Failed to parse --input JSON: ${error.message}`);
  }
}

function parseJSONOrSSE(text) {
  const trimmed = text.trim();
  if (trimmed.startsWith("{")) {
    return JSON.parse(trimmed);
  }
  const dataLine = trimmed
    .split(/\r?\n/)
    .find((line) => line.startsWith("data:"));
  if (!dataLine) {
    throw new Error(`MCP HTTP response was not JSON or SSE data: ${text}`);
  }
  return JSON.parse(dataLine.slice("data:".length).trim());
}

function assertIncludes(items, expected) {
  if (!items.includes(expected)) {
    throw new Error(`Expected tool ${expected}, got: ${items.join(", ")}`);
  }
}

function assertToolSuccess(response, toolName) {
  const result = response.result;
  if (!result?.isError) {
    return;
  }
  const message =
    result.content
      ?.map((item) => item.text)
      .filter(Boolean)
      .join("\n") || JSON.stringify(response);
  if (message.includes("not exposed to MCP")) {
    throw new Error(
      `${toolName} failed: ${message}. Enable Expose to MCP for the workflow, keep it active, and disable Requires Approval for MCP alpha/beta tests.`,
    );
  }
  throw new Error(`${toolName} failed: ${message}`);
}

function assertNoRawInputLeak(output) {
  const text = JSON.stringify(output);
  if (text.includes("input_json") || text.includes("mcp-http-secret-value")) {
    throw new Error(`MCP HTTP execution output leaked raw input: ${text}`);
  }
}

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    const item = argv[i];
    if (!item.startsWith("--")) {
      continue;
    }
    const key = item.slice(2).replace(/-([a-z])/g, (_, char) => char.toUpperCase());
    out[key] = argv[i + 1];
    i++;
  }
  return out;
}

function trimRight(value, char) {
  while (value.endsWith(char)) {
    value = value.slice(0, -1);
  }
  return value;
}
