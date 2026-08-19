import { describe, expect, it } from 'vitest';
import { credentialsForParam } from '../src/utils/inspector';

describe('credential compatibility metadata', () => {
  const credentials = [
    { id: 'zalo', type: 'BEARER_TOKEN', kind: 'BEARER_TOKEN', provider: 'zalo', name: 'Zalo OA' },
    { id: 'github', type: 'BEARER_TOKEN', kind: 'BEARER_TOKEN', provider: 'github', name: 'GitHub' },
    { id: 'openai', type: 'OpenAI', kind: 'API_KEY', provider: 'openai', name: 'OpenAI' },
    { id: 'legacy-telegram', type: 'TELEGRAM_BOT', name: 'Telegram legacy' },
    { id: 'basic', type: 'BASIC_AUTH', kind: 'BASIC_AUTH', provider: 'custom', name: 'Basic' },
  ];

  it('filters by generic authentication kind without requiring a service enum', () => {
    const param = {
      type: 'credential',
      credential_kinds: ['BEARER_TOKEN', 'API_KEY'],
    };
    expect(credentialsForParam(credentials, param, 'zaloOA').map((cred) => cred.id)).toEqual([
      'zalo',
      'github',
      'openai',
      'legacy-telegram',
    ]);
  });

  it('can optionally narrow a generic kind by provider metadata', () => {
    const param = {
      type: 'credential',
      credential_kinds: ['BEARER_TOKEN'],
      credential_providers: ['zalo'],
    };
    expect(credentialsForParam(credentials, param, 'zaloOA').map((cred) => cred.id)).toEqual(['zalo']);
  });

  it('keeps legacy node hint matching for definitions not migrated yet', () => {
    const param = { type: 'credential' };
    expect(credentialsForParam(credentials, param, 'telegramBot').map((cred) => cred.id)).toContain('legacy-telegram');
  });
});
