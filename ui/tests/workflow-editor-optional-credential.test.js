import { describe, expect, it } from 'vitest';
import {
  createWorkflowNode,
  validateWorkflowGraph,
  validationStateForNode,
} from '../src/utils/workflowEditor';

describe('optional credential validation', () => {
  it('does not flag an empty optional credential', () => {
    const httpDef = {
      type: 'httpRequest',
      name: 'HTTP Request',
      category: 'ACTION',
      params: [
        { name: 'url', label: 'URL', type: 'url', required: true, default: 'https://example.com' },
        { name: 'credential_id', label: 'Bearer/API Credential', type: 'credential', required: false, default: '' },
      ],
    };

    const node = createWorkflowNode(httpDef, { x: 0, y: 0 }, 'http');
    const issues = validateWorkflowGraph([node], [], [httpDef]);

    expect(issues.map((issue) => issue.type)).not.toContain('missing_credential');
    expect(validationStateForNode('http', issues)).toBe('Configured');
  });

  it('still flags an empty required credential', () => {
    const telegramDef = {
      type: 'telegramBot',
      name: 'Telegram Bot',
      category: 'COMMUNICATION',
      params: [
        { name: 'credential_id', label: 'Bot Credential', type: 'credential', required: true, default: '' },
      ],
    };

    const node = createWorkflowNode(telegramDef, { x: 0, y: 0 }, 'telegram');
    const issues = validateWorkflowGraph([node], [], [telegramDef]);

    expect(issues.map((issue) => issue.type)).toContain('missing_credential');
    expect(validationStateForNode('telegram', issues)).toBe('Missing credential');
  });
});
