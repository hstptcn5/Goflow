#!/usr/bin/env node
import { spawn } from "node:child_process";
import { setTimeout as sleep } from "node:timers/promises";

const args = parseArgs(process.argv.slice(2));
const binary = args.binary || (process.platform === "win32" ? ".\\goflow.exe" : "./goflow");
const url = args.url || process.env.GOFLOW_URL || "http://127.0.0.1:8080";
const apiKey = args.apiKey || process.env.GOFLOW_API_KEY || "";
const timeoutMs = Number(args.timeoutMs || 15000);
const workflow = args.workflow || "";
const dynamicTool = args.dynamicTool || "";
const expectTool = args.expectTool || "";
const input = parseInput(args.input || "{}");
const idempotencyKey = args.idempotencyKey || `mcp-smoke-${Date.now()}`;
const toolsOnly = args.toolsOnly === "true";

const mcp = spawn(binary, ["mcp", "stdio"], {
  cwd: process.cwd(),
  env: {
    ...process.env,
    GOFLOW_URL: url,
    GOFLOW_API_KEY: apiKey,
  },
  stdio: ["pipe", "pipe", "pipe"],
});

let stdout = "";
let stderr = "";
const responses = new Map();

mcp.stdout.on("data", (chunk) => {
  stdout += chunk.toString();
  for (const line of stdout.split(/\r?\n/).slice(0, -1)) {
    if (!line.trim()) {
      continue;
    }
    const message = JSON.parse(line);
    if (message.id !== undefined) {
      responses.set(message.id, message);
    }
  }
  stdout = stdout.includes("\n") ? stdout.slice(stdout.lastIndexOf("\n") + 1) : stdout;
});
mcp.stderr.on("data", (chunk) => {
  stderr += chunk.toString();
});

try {
  await request(1, "initialize", {
    protocolVersion: "2025-06-18",
    capabilities: {},
    clientInfo: { name: "goflow-mcp-smoke-test", version: "0.0.0" },
  });
  notify("notifications/initialized", {});

  const tools = await request(2, "tools/list", {});
  const toolNames = tools.result.tools.map((tool) => tool.name);
  assertIncludes(toolNames, "goflow_list_workflows");
  assertIncludes(toolNames, "goflow_run_workflow");
  assertIncludes(toolNames, "goflow_get_execution");
  assertIncludes(toolNames, "goflow_list_executions");
  assertIncludes(toolNames, "goflow_cancel_execution");
  if (expectTool) {
    assertIncludes(toolNames, expectTool);
  }
  console.log(`MCP tools/list passed (${toolNames.length} tools)`);

  if (!toolsOnly) {
    const workflowList = await request(3, "tools/call", {
      name: "goflow_list_workflows",
      arguments: { active_only: false },
    });
    const workflowCount = readStructuredContent(workflowList).count;
    console.log(`MCP goflow_list_workflows passed (${workflowCount} workflows)`);

    if (dynamicTool) {
      const run = await request(4, "tools/call", {
        name: dynamicTool,
        arguments: input,
      });
      const runOutput = readStructuredContent(run);
      if (!runOutput.execution_id) {
        throw new Error(`${dynamicTool} did not return execution_id`);
      }
      console.log(`MCP ${dynamicTool} passed (${runOutput.execution_id}, ${runOutput.status})`);
    } else if (workflow) {
      const run = await request(4, "tools/call", {
        name: "goflow_run_workflow",
        arguments: {
          workflow,
          input,
          idempotency_key: idempotencyKey,
        },
      });
      const runOutput = readStructuredContent(run);
      if (!runOutput.execution_id) {
        throw new Error("goflow_run_workflow did not return execution_id");
      }
      console.log(`MCP goflow_run_workflow passed (${runOutput.execution_id}, ${runOutput.status})`);

      const execution = await request(5, "tools/call", {
        name: "goflow_get_execution",
        arguments: { execution_id: runOutput.execution_id },
      });
      const executionOutput = readStructuredContent(execution);
      console.log(`MCP goflow_get_execution passed (${executionOutput.execution.status})`);
    }
  }
} finally {
  mcp.kill();
}

function send(message) {
  mcp.stdin.write(`${JSON.stringify(message)}\n`);
}

async function request(id, method, params) {
  send({ jsonrpc: "2.0", id, method, params });
  const started = Date.now();
  while (!responses.has(id)) {
    if (Date.now() - started > timeoutMs) {
      throw new Error(`Timed out waiting for MCP response ${id}. stderr: ${stderr.trim()}`);
    }
    await sleep(50);
  }
  const response = responses.get(id);
  if (response.error) {
    throw new Error(`MCP error for ${method}: ${JSON.stringify(response.error)}`);
  }
  return response;
}

function notify(method, params) {
  send({ jsonrpc: "2.0", method, params });
}

function readStructuredContent(response) {
  const content = response.result.structuredContent;
  if (!content || typeof content !== "object") {
    throw new Error(`MCP response has no structuredContent: ${JSON.stringify(response)}`);
  }
  return content;
}

function assertIncludes(items, expected) {
  if (!items.includes(expected)) {
    throw new Error(`Expected tool ${expected}, got: ${items.join(", ")}`);
  }
}

function parseInput(raw) {
  const parsed = JSON.parse(raw);
  if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
    throw new Error("--input must be a JSON object");
  }
  return parsed;
}

function parseArgs(argv) {
  const parsed = {};
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (!arg.startsWith("--")) {
      throw new Error(`Unexpected argument: ${arg}`);
    }
    const key = camelCase(arg.slice(2));
    const next = argv[index + 1];
    if (!next || next.startsWith("--")) {
      parsed[key] = "true";
      continue;
    }
    parsed[key] = next;
    index += 1;
  }
  return parsed;
}

function camelCase(value) {
  return value.replace(/-([a-z])/g, (_, char) => char.toUpperCase());
}
