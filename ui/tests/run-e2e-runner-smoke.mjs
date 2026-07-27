import { spawn } from 'node:child_process';
import { createServer } from 'node:net';
import { resolve } from 'node:path';

let server;
const port = await occupyPort();
const runner = resolve(process.cwd(), 'tests', 'run-e2e.mjs');

const result = await new Promise((resolveRun) => {
  const child = spawn(process.execPath, [runner, '--list'], {
    cwd: process.cwd(),
    env: {
      ...process.env,
      GOFLOW_E2E_PORT: String(port),
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let stdout = '';
  let stderr = '';
  child.stdout.on('data', (chunk) => { stdout += chunk; });
  child.stderr.on('data', (chunk) => { stderr += chunk; });
  child.on('exit', (code) => resolveRun({ code, stdout, stderr }));
});

server.close();

if (result.code === 0) {
  console.error('Expected run-e2e.mjs to fail when GOFLOW_E2E_PORT is occupied.');
  process.exit(1);
}
if (!/already in use|unavailable|EADDRINUSE/i.test(`${result.stdout}\n${result.stderr}`)) {
  console.error('Expected occupied-port error, got:');
  console.error(result.stdout);
  console.error(result.stderr);
  process.exit(1);
}

console.log(`run-e2e occupied-port smoke passed on port ${port}`);

function occupyPort() {
  return new Promise((resolvePort, rejectPort) => {
    server = createServer();
    server.once('error', rejectPort);
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      resolvePort(address.port);
    });
  });
}
