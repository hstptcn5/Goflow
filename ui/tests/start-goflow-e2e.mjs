import { spawn } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const root = resolve(process.cwd(), '..');
const tempDir = mkdtempSync(join(tmpdir(), 'goflow-ui-e2e-'));

const child = spawn('go', ['run', 'main.go', 'static_embed.go', 'serve'], {
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

function stop() {
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
