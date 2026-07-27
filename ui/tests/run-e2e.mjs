import { spawn } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync } from 'node:fs';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const uiRoot = process.cwd();
const root = resolve(uiRoot, '..');
const tempDir = mkdtempSync(join(tmpdir(), 'goflow-ui-e2e-'));
const explicitPort = Boolean(process.env.GOFLOW_E2E_PORT);
const port = explicitPort ? Number(process.env.GOFLOW_E2E_PORT) : await findFreePort();
if (!Number.isInteger(port) || port <= 0) throw new Error(`Invalid GOFLOW_E2E_PORT: ${process.env.GOFLOW_E2E_PORT}`);
if (explicitPort) await assertPortAvailable(port);
const baseURL = `http://127.0.0.1:${port}`;
const apiKey = `e2e-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
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
    GOFLOW_PORT: String(port),
    GOFLOW_API_KEY: apiKey,
    GOFLOW_DB_PATH: join(tempDir, 'goflow.db'),
    GOFLOW_MASTER_KEY_FILE: join(tempDir, 'goflow.master.key'),
    GOFLOW_HTTP_LOGS: '0',
  },
  stdio: 'inherit',
});

let serverExitedBeforeReady = false;
server.once('exit', (code, signal) => {
  serverExitedBeforeReady = true;
  if (!ready) {
    console.error(`Goflow E2E server exited before readiness: code=${code} signal=${signal}`);
  }
});
let ready = false;

function killTree(child) {
  return new Promise((resolveKill) => {
    if (!child || child.killed || child.exitCode !== null) {
      resolveKill();
      return;
    }
    const fallback = setTimeout(resolveKill, 5000);
    fallback.unref?.();
    const resolveOnce = () => {
      clearTimeout(fallback);
      resolveKill();
    };
    child.once('exit', resolveOnce);
    if (process.platform === 'win32' && child.pid) {
      const killer = spawn('taskkill', ['/pid', String(child.pid), '/T', '/F'], { stdio: 'ignore' });
      killer.once('exit', () => {
        if (child.exitCode !== null) resolveOnce();
      });
      return;
    }
    child.kill('SIGTERM');
  });
}

async function findFreePort() {
  return new Promise((resolvePort, rejectPort) => {
    const server = createServer();
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      const found = address && typeof address === 'object' ? address.port : 0;
      server.close(() => resolvePort(found));
    });
    server.on('error', rejectPort);
  });
}

async function assertPortAvailable(candidatePort) {
  return new Promise((resolvePort, rejectPort) => {
    const server = createServer();
    server.once('error', (err) => rejectPort(new Error(`GOFLOW_E2E_PORT ${candidatePort} is already in use or unavailable: ${err.message}`)));
    server.listen(candidatePort, '127.0.0.1', () => {
      server.close(resolvePort);
    });
  });
}

async function waitForServer() {
  const url = `${baseURL}/api/v1/workflows`;
  for (let i = 0; i < 120; i += 1) {
    if (serverExitedBeforeReady) throw new Error('Goflow E2E server exited before readiness');
    try {
      const res = await fetch(url, { headers: { Authorization: `Bearer ${apiKey}` } });
      if (res.ok) {
        ready = true;
        return;
      }
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
        GOFLOW_E2E_BASE_URL: baseURL,
        GOFLOW_E2E_API_KEY: apiKey,
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
  await killTree(server);
  await removeTempDir(tempDir);
}

process.exit(exitCode);

async function removeTempDir(path) {
  for (let attempt = 0; attempt < 10; attempt += 1) {
    try {
      rmSync(path, { recursive: true, force: true });
      return;
    } catch (err) {
      if (attempt === 9) {
        console.warn(`Could not remove E2E temp directory ${path}: ${err.message}`);
        return;
      }
      await new Promise((resolveRetry) => setTimeout(resolveRetry, 250));
    }
  }
}
