import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';

const activationSource = readFileSync(new URL('../src/components/ApplianceActivationProbe.vue', import.meta.url), 'utf8');
const appSource = readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8');

describe('first-customer activation surface', () => {
  it('mounts only alongside appliance mode', () => {
    expect(appSource).toContain('ApplianceActivationProbe');
    expect(appSource).toContain('<template v-if="applianceBootstrap">');
  });

  it('celebrates a successful manual first run without exporting private data', () => {
    expect(activationSource).toContain("value?.trigger_source === 'ui'");
    expect(activationSource).toContain("['SUCCESS', 'SUCCEEDED', 'COMPLETED']");
    expect(activationSource).toContain('Your first automation is working.');
    expect(activationSource).toContain('Quy trình đầu tiên của bạn đã chạy thành công.');
    expect(activationSource).toContain('goflow-activation-complete:');
    expect(activationSource).not.toMatch(/analytics|telemetry|sendBeacon|external/i);
  });
});
