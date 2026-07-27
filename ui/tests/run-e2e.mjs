import { spawn } from 'node:child_process';
import { mkdirSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const uiRoot = process.cwd();
const root = resolve(uiRoot, '..');
const tempDir = mkdtempSync(join(tmpdir(), 'goflow-ui-e2e-'));
const port = process.env.GOFLOW_E2E_PORT || '18081';
const configuredBinary = process.env.GOFLOW_E2E_BINARY;
const serverCommand = configuredBinary ? resolve(root, configuredBinary) : 'go';
const serverArgs = configuredBinary ? ['serve'] : ['run', 'main.go', 'static_embed.go', 'serve'];
const playwrightBin = process.platform === 'win32'
  ? join(uiRoot, 'node_modules', '.bin', 'playwright.cmd')
  : join(uiRoot, 'node_modules', '.bin', 'playwright');

mkdirSync(join(root, '.gocache'), { recursive: true });

const server = spawn(serverCommand, serverArgs, {
  cwd: root,
  env: {
    ...process.env,
    GOCACHE: process.env.GOCACHE || join(root, '.gocache'),
    GOFLOW_HOST: '127.0.0.1',
    GOFLOW_PORT: port,
    GOFLOW_DB_PATH: join(tempDir, 'goflow.db'),
    GOFLOW_MASTER_KEY_FILE: join(tempDir, 'goflow.master.key'),
    GOFLOW_HTTP_LOGS: '0',
  },
  stdio: 'inherit',
});

function killTree(child) {
  if (!child || child.killed) return;
  if (process.platform === 'win32' && child.pid) {
    spawn('taskkill', ['/pid', String(child.pid), '/T', '/F'], { stdio: 'ignore' });
    return;
  }
  child.kill('SIGTERM');
}

async function waitForServer() {
  const url = `http://127.0.0.1:${port}/workflows`;
  for (let i = 0; i < 120; i += 1) {
    try {
      const res = await fetch(url);
      if (res.ok) return;
    } catch {
      // Retry until the Go server is ready.
    }
    await new Promise((resolveWait) => setTimeout(resolveWait, 500));
  }
  throw new Error(`Goflow E2E server did not become ready at ${url}`);
}

function runPlaywright() {
  return new Promise((resolveRun) => {
    const playwrightArgs = ['test', ...process.argv.slice(2)];
    const command = process.platform === 'win32' ? (process.env.ComSpec || 'cmd.exe') : playwrightBin;
    const args = process.platform === 'win32'
      ? ['/d', '/s', '/c', [playwrightBin, ...playwrightArgs].map(quoteCmdArg).join(' ')]
      : playwrightArgs;
    const child = spawn(command, args, {
      cwd: uiRoot,
      env: {
        ...process.env,
        GOFLOW_E2E_NO_WEBSERVER: '1',
      },
      stdio: 'inherit',
    });
    child.on('exit', (code) => resolveRun(code ?? 1));
  });
}

function quoteCmdArg(value) {
  const text = String(value);
  if (!/[\s"]/u.test(text)) return text;
  return `"${text.replace(/"/g, '\\"')}"`;
}

let exitCode = 1;
try {
  await waitForServer();
  exitCode = await runPlaywright();
} catch (err) {
  console.error(err);
} finally {
  killTree(server);
}

setTimeout(() => process.exit(exitCode), 500);
