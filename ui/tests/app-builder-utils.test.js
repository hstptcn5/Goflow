import { describe, expect, it } from 'vitest';
import { defaultInputMode, fieldsFromJSONSchema } from '../src/utils/appBuilder';
import { buildRunInput, outputView } from '../src/utils/applianceRunUi';

describe('app builder contracts', () => {
  it('derives a focused form from workflow input schema', () => {
    const fields = fieldsFromJSONSchema({
      type: 'object', required: ['amount'], properties: {
        amount: { type: 'number', title: 'Số tiền' },
        numbers: { type: 'array', items: { type: 'number' } },
      },
    });
    expect(fields).toMatchObject([{ key: 'amount', type: 'number', required: true }, { key: 'numbers', type: 'number_list' }]);
    expect(defaultInputMode([{ data: { type: 'webhookTrigger' } }])).toBe('webhook_body');
  });

  it('coerces form values into the webhook body contract', () => {
    const result = buildRunInput([
      { key: 'amount', label: 'Số tiền', type: 'number', required: true },
      { key: 'numbers', label: 'Numbers', type: 'number_list' },
    ], { amount: '25.5', numbers: '10, 20, 30' }, 'webhook_body');
    expect(result.errors).toEqual({});
    expect(result.input).toEqual({ body: { amount: 25.5, numbers: [10, 20, 30] } });
  });

  it('selects cards, table, or JSON without hiding data', () => {
    expect(outputView({ count: 3, average: 20 }, 'auto').mode).toBe('cards');
    expect(outputView([{ id: 1 }, { id: 2 }], 'auto')).toMatchObject({ mode: 'table', columns: ['id'] });
    expect(outputView({ nested: { ok: true } }, 'auto').mode).toBe('json');
  });
});
