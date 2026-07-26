#!/usr/bin/env node

const args = parseArgs(process.argv.slice(2));
const url = trimRight(args.url || process.env.GOFLOW_MCP_URL || "http://127.0.0.1:8080/mcp", "/");
const apiKey = args.apiKey || process.env.GOFLOW_API_KEY || "";
const origin = args.origin || "";

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
console.log(`MCP HTTP tools/list passed (${toolNames.length} tools)`);

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
