import { describe, expect, it, vi } from 'vitest';
import NodePicker from '../src/components/NodePicker.vue';
import { useWorkflowStore } from '../src/stores/workflowStore';
import { mountWithApp, nextFrame } from './mount';

const nodeDefs = [
  { type: 'manualTrigger', name: 'Manual Trigger', category: 'TRIGGER', description: 'Start manually', params: [] },
  { type: 'httpRequest', name: 'HTTP Request', category: 'ACTION', description: 'Send API request', params: [] },
  { type: 'telegramBot', name: 'Telegram Bot', category: 'COMMUNICATION', description: 'Send message', params: [{ name: 'credential_id', type: 'credential', required: true }] },
];

describe('NodePicker', () => {
  it('opens focused, searches by name/type/description, and shows empty state', async () => {
    const focusSpy = vi.spyOn(HTMLElement.prototype, 'focus');
    const { root } = await mountWithApp(NodePicker, { props: { visible: true } });
    const store = useWorkflowStore();
    store.nodeDefinitions = nodeDefs;
    await nextFrame();

    const input = root.querySelector('[aria-label="Search nodes"]');
    expect(input).not.toBeNull();
    expect(focusSpy).toHaveBeenCalled();

    input.value = 'api';
    input.dispatchEvent(new Event('input'));
    await nextFrame();
    expect(root.textContent).toContain('HTTP Request');

    input.value = 'manualTrigger';
    input.dispatchEvent(new Event('input'));
    await nextFrame();
    expect(root.textContent).toContain('Manual Trigger');

    input.value = 'does-not-exist';
    input.dispatchEvent(new Event('input'));
    await nextFrame();
    expect(root.textContent).toContain('No nodes match this search.');
  });

  it('supports keyboard selection and local favorite/recent state', async () => {
    const { root } = await mountWithApp(NodePicker, { props: { visible: true } });
    const store = useWorkflowStore();
    store.nodeDefinitions = nodeDefs;
    await nextFrame();

    root.querySelector('[aria-label="Search nodes"]').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    root.querySelector('[aria-label="Search nodes"]').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    await nextFrame();
    expect(JSON.parse(localStorage.getItem('goflow.recentNodes'))).toContain('httpRequest');

    root.querySelector('[aria-label="Add HTTP Request to favorites"]').click();
    await nextFrame();
    expect(JSON.parse(localStorage.getItem('goflow.favoriteNodes'))).toContain('httpRequest');
  });
});
