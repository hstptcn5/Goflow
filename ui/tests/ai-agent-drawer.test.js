import { afterEach, describe, expect, it, vi } from 'vitest';
import AIAssistantDrawer from '../src/components/AIAssistantDrawer.vue';
import { api } from '../src/services/api';
import * as aiAgentClient from '../src/services/aiAgent';
import { useWorkflowStore } from '../src/stores/workflowStore';
import { mountWithApp, nextFrame } from './mount';

afterEach(() => {
  vi.restoreAllMocks();
  window.localStorage.clear();
});

function findButton(root, text) {
  return Array.from(root.querySelectorAll('button')).find((button) => button.textContent.trim().includes(text));
}

describe('AI Agent Lab drawer', () => {
  it('returns a validated proposal without saving or production-running it automatically', async () => {
    vi.spyOn(api, 'getCredentials').mockResolvedValue([{
      id: 'deepseek-agent',
      name: 'DeepSeek Agent',
      type: 'API_KEY',
      kind: 'API_KEY',
      provider: 'deepseek',
    }]);
    const iterateSpy = vi.spyOn(aiAgentClient, 'iterateAIAgent').mockResolvedValue({
      summary: 'Đã đơn giản hóa nhánh điều kiện.',
      expected_improvement: 'Ít node hơn và dễ debug hơn.',
      proposal_validated: true,
      iterations: 1,
      test_status: 'passed',
      test_execution: { status: 'SUCCESS' },
      proposed_workflow: {
        name: 'Agent proposal',
        nodes: [{ id: 'manual_1', type: 'manualTrigger', name: 'Bắt đầu', params: {} }],
        edges: [],
      },
    });
    const updateSpy = vi.spyOn(api, 'updateWorkflow').mockResolvedValue({});
    const triggerSpy = vi.spyOn(api, 'triggerWorkflow').mockResolvedValue({});
    let loaded = null;

    const { root } = await mountWithApp(AIAssistantDrawer, {
      props: {
        visible: true,
        currentNodes: [{ id: 'manual_1', data: { type: 'manualTrigger', name: 'Bắt đầu', params: {} }, position: { x: 0, y: 0 } }],
        currentEdges: [],
        onLoadWorkflow: (workflow) => { loaded = workflow; },
      },
    });
    const workflowStore = useWorkflowStore();
    workflowStore.currentWorkflow = { id: 'wf-agent', name: 'Agent test' };
    await nextFrame();

    findButton(root, 'Agent Lab').click();
    await nextFrame();
    const textarea = root.querySelector('.prompt-input');
    textarea.value = 'Làm workflow này đơn giản hơn';
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
    await nextFrame();

    const runAgentButton = findButton(root, 'Chạy Agent');
    expect(runAgentButton).toBeTruthy();
    expect(runAgentButton.disabled).toBe(false);
    runAgentButton.click();
    await nextFrame();
    await nextFrame();

    expect(iterateSpy).toHaveBeenCalledTimes(1);
    expect(root.textContent).toContain('Safe test: PASS');
    expect(root.textContent).toContain('Đã đơn giản hóa nhánh điều kiện.');
    expect(updateSpy).not.toHaveBeenCalled();
    expect(triggerSpy).not.toHaveBeenCalled();
    expect(loaded).toBeNull();

    findButton(root, 'Nạp proposal lên canvas').click();
    await nextFrame();
    expect(loaded).toMatchObject({ name: 'Agent proposal' });
    expect(updateSpy).not.toHaveBeenCalled();
    expect(triggerSpy).not.toHaveBeenCalled();
  });
});
