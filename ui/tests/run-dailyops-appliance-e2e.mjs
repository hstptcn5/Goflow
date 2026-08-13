import { spawn } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync } from 'node:fs';
import { createServer as createHTTPServer } from 'node:http';
import { tmpdir } from 'node:os';
import { basename, join, resolve } from 'node:path';

const uiRoot = process.cwd();
const root = resolve(uiRoot, '..');
const tempDir = mkdtempSync(join(tmpdir(), 'goflow-dailyops-e2e-'));
const dataDir = join(tempDir, 'data');
const artifactDir = join(tempDir, 'artifacts');
const packDir = join(root, 'examples', 'packs', 'dailyops-rest-telegram');
const uiDir = join(uiRoot, 'dist');
const configuredHarnessBinary = process.env.GOFLOW_DAILYOPS_HARNESS_BINARY;
const playwrightBin = process.platform === 'win32'
  ? join(uiRoot, 'node_modules', '.bin', 'playwright.cmd')
  : join(uiRoot, 'node_modules', '.bin', 'playwright');
const tokenPrefix = '999999';
const tokenSecret = randomBytes(18).toString('base64url');
const tokenParts = `${tokenPrefix}|${tokenSecret}`;
const chatID = '@dailyops_e2e';
const capturedLogs = [];
const expectedMessageFragments = [
  '2026-08-09',
  '48250.75',
  '314',
  '3 SKUs below threshold',
  'Revenue up 12.4% vs prior day',
];

mkdirSync(dataDir, { recursive: true });
mkdirSync(artifactDir, { recursive: true });
mkdirSync(join(root, '.gocache'), { recursive: true });

const harnessBinary = configuredHarnessBinary || await buildHarness();
const sourceServer = await startSourceServer();
const telegramServer = await startTelegramServer();
const sourceURL = `${sourceServer.url}/dailyops.json`;
const telegramBaseURL = telegramServer.url;

let exitCode = 1;
let harness = null;
try {
  harness = await startHarness();
  exitCode = await runPlaywright('setup', harness.baseURL, harness.controlURL);
  if (exitCode !== 0) throw new Error('DailyOps setup Playwright phase failed');
  await killTree(harness.child);

  assertSourceCalls(sourceServer, { count: 5, dailyops: 3, html: 1, missing: 1 });
  assertTelegramCalls(telegramServer, { getMe: 4, getChat: 3, sendMessage: 1 });

  harness = await startHarness();
  assertSourceCalls(sourceServer, { count: 5, dailyops: 3, html: 1, missing: 1 });
  assertTelegramCalls(telegramServer, { getMe: 4, getChat: 3, sendMessage: 1 });
  exitCode = await runPlaywright('persist', harness.baseURL, harness.controlURL);
  if (exitCode !== 0) throw new Error('DailyOps persistence Playwright phase failed');
  assertSourceCalls(sourceServer, { count: 6, dailyops: 4, html: 1, missing: 1 });
  assertTelegramCalls(telegramServer, { getMe: 4, getChat: 3, sendMessage: 2 });

  await scanForForbiddenRuntimeData();
  ensureLogsDoNotExposeSecret();
  exitCode = 0;
} catch (err) {
  console.error(String(err.message || err));
} finally {
  if (harness?.child) await killTree(harness.child);
  await closeServer(sourceServer.server);
  await closeServer(telegramServer.server);
  await removeTempDir(tempDir);
}

process.exit(exitCode);

async function startSourceServer() {
  const state = { count: 0, methods: [], paths: [], unexpected: [], holdDailyops: false, releaseDailyops: null };
  const server = createHTTPServer(async (req, res) => {
    const pathname = new URL(req.url || '/', 'http://127.0.0.1').pathname;
    if (req.method === 'POST' && pathname === '/__control/hold') {
      state.holdDailyops = true;
      res.writeHead(204);
      res.end();
      return;
    }
    if (req.method === 'POST' && pathname === '/__control/release') {
      state.holdDailyops = false;
      state.releaseDailyops?.();
      state.releaseDailyops = null;
      res.writeHead(204);
      res.end();
      return;
    }
    state.count += 1;
    state.methods.push(req.method);
    state.paths.push(req.url);
    if (req.method !== 'GET' || !['/dailyops.json', '/html', '/missing'].includes(pathname)) {
      state.unexpected.push({ method: req.method, path: req.url || '' });
      res.writeHead(404);
      res.end('not found');
      return;
    }
    if (pathname === '/html') {
      res.writeHead(200, { 'Content-Type': 'text/html' });
      res.end('<html><body>private dashboard</body></html>');
      return;
    }
    if (pathname === '/missing') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ report_date: '2026-08-09', timezone: 'Asia/Bangkok' }));
      return;
    }
    if (state.holdDailyops) {
      await new Promise((resolveRelease) => {
        state.releaseDailyops = resolveRelease;
      });
    }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      report_date: '2026-08-09',
      timezone: 'Asia/Bangkok',
      revenue: 48250.75,
      order_count: 314,
      cancelled_refunded_count: 7,
      low_stock_summary: '3 SKUs below threshold',
      comparison_summary: 'Revenue up 12.4% vs prior day',
    }));
  });
  const listened = await listen(server);
  return { ...listened, state };
}

async function startTelegramServer() {
  const state = { getMe: 0, getChat: 0, sendMessage: 0, unexpected: [] };
  const server = createHTTPServer(async (req, res) => {
    const expectedPrefix = `/bot${tokenPrefix}:${tokenSecret}/`;
    const requestURL = new URL(req.url || '/', 'http://127.0.0.1');
    const method = req.url?.startsWith(expectedPrefix) ? requestURL.pathname.slice(expectedPrefix.length) : requestURL.pathname.split('/').pop();
    if (req.method === 'GET' && method === 'getMe') {
      state.getMe += 1;
      if (!req.url?.startsWith(expectedPrefix)) {
        res.writeHead(401, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ ok: false, description: 'invalid token' }));
        return;
      }
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ ok: true, result: { id: 42, is_bot: true, username: 'dailyops_e2e_bot' } }));
      return;
    }
    if (req.method === 'GET' && method === 'getChat' && req.url?.startsWith(expectedPrefix)) {
      state.getChat += 1;
      if (requestURL.searchParams.get('chat_id') !== chatID) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ ok: false, description: 'chat not found' }));
        return;
      }
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ ok: true, result: { id: chatID, type: 'channel' } }));
      return;
    }
    if (req.method === 'POST' && method === 'sendMessage') {
      state.sendMessage += 1;
      const body = await readBody(req);
      const payload = JSON.parse(body || '{}');
      const text = String(payload.text || '');
      const missingFragments = expectedMessageFragments.filter((fragment) => !text.includes(fragment));
      if (payload.chat_id !== chatID || !text.includes('DailyOps Daily Report') || missingFragments.length > 0) {
        state.unexpected.push({ method: req.method, path: 'sendMessage', reason: missingFragments.length ? `missing source fragments: ${missingFragments.join(', ')}` : 'invalid payload' });
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ ok: false, description: 'invalid payload' }));
        return;
      }
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ ok: true, result: { message_id: 101, chat: { id: chatID }, text: payload.text } }));
      return;
    }
    state.unexpected.push({ method: req.method, path: method });
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ ok: false, description: 'unsupported method' }));
  });
  const listened = await listen(server);
  return { ...listened, state };
}

function listen(server) {
  return new Promise((resolveListen, rejectListen) => {
    server.once('error', rejectListen);
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      const port = address && typeof address === 'object' ? address.port : 0;
      resolveListen({ server, url: `http://127.0.0.1:${port}` });
    });
  });
}

async function startHarness() {
  const harnessArgs = [
    '--pack-dir', packDir,
    '--data-dir', dataDir,
    '--ui-dir', uiDir,
    '--telegram-base-url', telegramBaseURL,
    '--port', '0',
  ];
  const child = spawn(harnessBinary, harnessArgs, {
    cwd: root,
    env: {
      ...process.env,
      GOCACHE: process.env.GOCACHE || join(root, '.gocache'),
      GOFLOW_HTTP_LOGS: '0',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const harnessURLsPromise = waitForHarnessURLs(child);
  child.stderr.on('data', (chunk) => recordLog(chunk));
  const { baseURL, controlURL } = await harnessURLsPromise;
  await waitForAppliance(child, baseURL);
  return { child, baseURL, controlURL };
}

async function buildHarness() {
  const binary = join(tempDir, process.platform === 'win32' ? 'dailyops-appliance-harness.exe' : 'dailyops-appliance-harness');
  await runCommand('go', ['build', '-trimpath', '-o', binary, './internal/testharness/dailyopsappliance'], root);
  return binary;
}

function waitForHarnessURLs(child) {
  return new Promise((resolveURL, rejectURL) => {
    let buffer = '';
    let settled = false;
    let baseURL = '';
    let controlURL = '';
    const timeout = setTimeout(() => {
      if (!settled) {
        settled = true;
        rejectURL(new Error('DailyOps appliance harness did not print a URL'));
      }
    }, 60_000);
    child.stdout.on('data', (chunk) => {
      const text = recordLog(chunk);
      buffer += text;
      baseURL ||= buffer.match(/URL:\s+(http:\/\/127\.0\.0\.1:\d+)\//)?.[1] || '';
      controlURL ||= buffer.match(/CONTROL:\s+(http:\/\/127\.0\.0\.1:\d+)/)?.[1] || '';
      if (baseURL && controlURL && !settled) {
        settled = true;
        clearTimeout(timeout);
        resolveURL({ baseURL, controlURL });
      }
    });
    child.once('exit', (code, signal) => {
      if (!settled) {
        settled = true;
        clearTimeout(timeout);
        rejectURL(new Error(`DailyOps appliance harness exited before URL: code=${code} signal=${signal}`));
      }
    });
  });
}

function runPlaywright(phaseName, baseURL, controlURL) {
  return new Promise((resolveRun) => {
    const command = process.platform === 'win32' ? (process.env.ComSpec || 'cmd.exe') : playwrightBin;
    const args = process.platform === 'win32'
      ? ['/d', '/s', '/c', [playwrightBin, 'test', 'dailyops-appliance-real.spec.js'].map(quoteCmdArg).join(' ')]
      : ['test', 'dailyops-appliance-real.spec.js'];
    const child = spawn(command, args, {
      cwd: uiRoot,
      env: {
        ...process.env,
        GOFLOW_E2E_NO_WEBSERVER: '1',
        GOFLOW_E2E_BASE_URL: baseURL,
        GOFLOW_DAILYOPS_PHASE: phaseName,
        GOFLOW_DAILYOPS_SOURCE_URL: sourceURL,
        GOFLOW_DAILYOPS_CHAT_ID: chatID,
        GOFLOW_DAILYOPS_TOKEN_PARTS: tokenParts,
        GOFLOW_DAILYOPS_SCHEDULE_CONTROL_URL: controlURL,
        GOFLOW_DAILYOPS_SOURCE_CONTROL_URL: sourceServer.url,
      },
      stdio: 'inherit',
    });
    child.on('exit', (code) => resolveRun(code ?? 1));
  });
}

async function waitForAppliance(child, baseURL) {
  let exited = false;
  child.once('exit', () => {
    exited = true;
  });
  for (let i = 0; i < 120; i += 1) {
    if (exited) throw new Error('DailyOps appliance harness exited before readiness');
    try {
      const res = await fetch(`${baseURL}/api/appliance/bootstrap`);
      if (res.ok) return;
    } catch {
      // Retry until the harness starts listening.
    }
    await new Promise((resolveWait) => setTimeout(resolveWait, 500));
  }
  throw new Error(`DailyOps appliance harness did not become ready at ${baseURL}`);
}

function assertTelegramCalls(serverState, expected) {
  if (serverState.state.unexpected.length) {
    throw new Error(`Unexpected Telegram mock calls: ${JSON.stringify(serverState.state.unexpected)}`);
  }
  if (serverState.state.getMe !== expected.getMe || serverState.state.getChat !== expected.getChat || serverState.state.sendMessage !== expected.sendMessage) {
    throw new Error(`Telegram mock call counts mismatch: getMe=${serverState.state.getMe}, getChat=${serverState.state.getChat}, sendMessage=${serverState.state.sendMessage}`);
  }
}

function assertSourceCalls(serverState, expected) {
  const { state } = serverState;
  if (state.unexpected.length) {
    throw new Error(`Unexpected source mock calls: ${JSON.stringify(state.unexpected)}`);
  }
  if (state.count !== expected.count) {
    throw new Error(`Source mock call count mismatch: count=${state.count}, methods=${state.methods.join(',')}, paths=${state.paths.join(',')}`);
  }
  if (state.methods.some((method) => method !== 'GET')) throw new Error(`Source mock request method mismatch: ${state.methods.join(',')}`);
  const counts = state.paths.reduce((result, rawPath) => {
    const pathname = new URL(rawPath, 'http://127.0.0.1').pathname;
    result[pathname] = (result[pathname] || 0) + 1;
    return result;
  }, {});
  if (counts['/dailyops.json'] !== expected.dailyops || counts['/html'] !== expected.html || counts['/missing'] !== expected.missing) {
    throw new Error(`Source mock path counts mismatch: ${JSON.stringify(counts)}`);
  }
}

async function scanForForbiddenRuntimeData() {
  const bundlePath = await buildDailyOpsBundle();
  const extractedDir = join(artifactDir, 'extracted');
  await extractZip(bundlePath, extractedDir);
  const scanRoots = [packDir, extractedDir, join(uiRoot, 'test-results')].filter(existsSync);
  const forbidden = [tokenSecret, sourceURL, telegramBaseURL, dataDir, tempDir, harness?.controlURL].filter(Boolean);
  const hits = [];
  for (const rootPath of scanRoots) {
    for (const filePath of walk(rootPath)) {
      const relative = filePath.slice(rootPath.length + 1);
      if (/\.(db|sqlite|sqlite3|key|pem|p12|zip|exe|dll|so|dylib)$/i.test(basename(filePath))) {
        if (rootPath === packDir) hits.push(`${relative}: forbidden runtime artifact type`);
        continue;
      }
      const data = readFileSync(filePath);
      for (const value of forbidden) {
        if (value && data.includes(Buffer.from(value))) hits.push(`${relative}: contains forbidden runtime value`);
      }
    }
  }
  if (hits.length) throw new Error(`Forbidden runtime data scan failed: ${hits.join('; ')}`);
}

function buildDailyOpsBundle() {
  return new Promise((resolveBuild, rejectBuild) => {
    const child = spawn('go', ['run', '.', 'pack', 'build', packDir, '--output', artifactDir, '--force'], {
      cwd: root,
      env: { ...process.env, GOCACHE: process.env.GOCACHE || join(root, '.gocache') },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let stderr = '';
    child.stderr.on('data', (chunk) => {
      stderr += chunk.toString('utf8');
    });
    child.on('exit', (code) => {
      if (code !== 0) {
        rejectBuild(new Error(`DailyOps bundle build failed: ${stderr}`));
        return;
      }
      const zip = readdirSync(artifactDir).find((name) => name.endsWith('.zip'));
      if (!zip) {
        rejectBuild(new Error('DailyOps bundle build did not produce a ZIP'));
        return;
      }
      resolveBuild(join(artifactDir, zip));
    });
  });
}

async function extractZip(zipPath, destination) {
  mkdirSync(destination, { recursive: true });
  if (process.platform === 'win32') {
    return runCommand(process.env.ComSpec || 'cmd.exe', ['/d', '/s', '/c', `powershell -NoProfile -Command "Expand-Archive -LiteralPath '${zipPath.replace(/'/g, "''")}' -DestinationPath '${destination.replace(/'/g, "''")}' -Force"`], root);
  }
  try {
    await runCommand('unzip', ['-q', zipPath, '-d', destination], root);
  } catch {
    await runCommand('python3', ['-m', 'zipfile', '-e', zipPath, destination], root);
  }
}

function runCommand(command, args, cwd) {
  return new Promise((resolveRun, rejectRun) => {
    const child = spawn(command, args, { cwd, stdio: 'ignore' });
    child.on('exit', (code) => {
      if (code === 0) resolveRun();
      else rejectRun(new Error(`${command} exited with ${code}`));
    });
  });
}

function walk(rootPath) {
  const files = [];
  for (const entry of readdirSync(rootPath, { withFileTypes: true })) {
    const path = join(rootPath, entry.name);
    if (entry.isDirectory()) {
      if (['node_modules', '.git'].includes(entry.name)) continue;
      files.push(...walk(path));
    } else if (entry.isFile()) {
      files.push(path);
    }
  }
  return files;
}

function recordLog(chunk) {
  const text = chunk.toString('utf8');
  capturedLogs.push(text);
  process.stdout.write(text.replaceAll(tokenSecret, '[REDACTED]'));
  return text;
}

function ensureLogsDoNotExposeSecret() {
  if (capturedLogs.join('').includes(tokenSecret)) {
    throw new Error('Harness logs exposed the fake Telegram token');
  }
}

function readBody(req) {
  return new Promise((resolveRead, rejectRead) => {
    let body = '';
    req.setEncoding('utf8');
    req.on('data', (chunk) => {
      body += chunk;
      if (body.length > 64 * 1024) {
        req.destroy(new Error('request too large'));
      }
    });
    req.on('end', () => resolveRead(body));
    req.on('error', rejectRead);
  });
}

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

function closeServer(server) {
  return new Promise((resolveClose) => server.close(resolveClose));
}

async function removeTempDir(path) {
  for (let attempt = 0; attempt < 10; attempt += 1) {
    try {
      rmSync(path, { recursive: true, force: true });
      return;
    } catch (err) {
      if (attempt === 9) {
        console.warn(`Could not remove DailyOps E2E temp directory: ${err.message}`);
        return;
      }
      await new Promise((resolveRetry) => setTimeout(resolveRetry, 250));
    }
  }
}

function quoteCmdArg(value) {
  const text = String(value);
  if (!/[\s"]/u.test(text)) return text;
  return `"${text.replace(/"/g, '\\"')}"`;
}
