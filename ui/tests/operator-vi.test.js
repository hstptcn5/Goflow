import { describe, expect, it } from 'vitest';
import { localizeNodeDefinitions, translateOperatorText, viNodeName } from '../src/utils/operatorVi';

describe('Vietnamese operator localization', () => {
  it('localizes node names, descriptions and parameter labels without changing technical identifiers', () => {
    const [definition] = localizeNodeDefinitions([{
      type: 'httpRequest',
      name: 'HTTP Request',
      description: 'Sends HTTP requests',
      params: [
        { name: 'headers', label: 'Headers (JSON)', description: 'Headers', type: 'json' },
        { name: 'url', label: 'Target URL', description: 'URL', type: 'text' },
      ],
    }]);

    expect(definition.type).toBe('httpRequest');
    expect(definition.name).toBe('Yêu cầu HTTP');
    expect(definition.params[0].name).toBe('headers');
    expect(definition.params[0].label).toBe('Header (JSON)');
    expect(definition.params[1].name).toBe('url');
    expect(definition.params[1].label).toBe('URL đích');
  });

  it('covers Checkpoint N node names', () => {
    expect(viNodeName('sourcePolicy')).toBe('Chính sách nguồn');
    expect(viNodeName('aiExtract')).toBe('AI Trích xuất');
    expect(viNodeName('zaloOA')).toBe('Zalo OA');
  });

  it('translates common operator labels and dynamic execution labels', () => {
    expect(translateOperatorText('Save')).toBe('Lưu');
    expect(translateOperatorText('Latest execution abc123')).toBe('Lần chạy gần nhất abc123');
    expect(translateOperatorText('5 findings')).toBe('5 phát hiện');
  });
});
