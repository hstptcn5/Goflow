import { spawn } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const root = resolve(process.cwd(), '..');
const tempDir = mkdtempSync(join(tmpdir(), 'goflow-ui-e2e-'));
const configuredBinary = process.env.GOFLOW_E2E_BINARY;
const command = configuredBinary ? resolve(root, configuredBinary) : 'go';
const args = configuredBinary ? ['serve'] : ['run', 'main.go', 'static_embed.go', 'serve'];

const child = spawn(command, args, {
  cwd: root,
  env: {
    ...process.env,
    GOFLOW_HOST: '127.0.0.1',
    GOFLOW_PORT: '18081',
    GOFLOW_DB_PATH: join(tempDir, 'goflow.db'),
    GOFLOW_MASTER_KEY_FILE: join(tempDir, 'goflow.master.key'),
    GOFLOW_HTTP_LOGS: '0',
  },
  stdio: 'inherit',
});

let stopping = false;

function stop() {
  if (stopping) return;
  stopping = true;
  const forceExit = setTimeout(() => process.exit(0), 1000);
  forceExit.unref?.();
  if (child.killed) return;
  if (process.platform === 'win32' && child.pid) {
    spawn('taskkill', ['/pid', String(child.pid), '/T', '/F'], { stdio: 'ignore' });
    return;
  }
  if (!child.killed) {
    child.kill('SIGTERM');
  }
}

process.on('SIGINT', stop);
process.on('SIGTERM', stop);
process.on('exit', stop);

child.on('exit', (code) => {
  process.exit(code ?? 0);
});
