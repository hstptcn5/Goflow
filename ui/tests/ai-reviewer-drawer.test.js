import { afterEach, describe, expect, it, vi } from 'vitest';
import AIAssistantDrawer from '../src/components/AIAssistantDrawer.vue';
import { api } from '../src/services/api';
import { useExecutionStore } from '../src/stores/executionStore';
import { useWorkflowStore } from '../src/stores/workflowStore';
import { mountWithApp, nextFrame } from './mount';

afterEach(() => {
  vi.restoreAllMocks();
});

function findButton(root, text) {
  return Array.from(root.querySelectorAll('button')).find((button) => button.textContent.trim().includes(text));
}

describe('AI workflow reviewer drawer', () => {
  it('uses the latest execution from the execution store and never saves or runs automatically', async () => {
    const credential = {
      id: 'openai-reviewer',
      name: 'OpenAI Reviewer',
      type: 'API_KEY',
      kind: 'API_KEY',
      provider: 'openai',
    };
    vi.spyOn(api, 'getCredentials').mockResolvedValue([credential]);
    const reviewSpy = vi.spyOn(api, 'reviewAIWorkflow').mockResolvedValue({
      mode: 'latest_run',
      provider: 'openai',
      model: 'gpt-4o',
      summary: 'The latest output can be improved.',
      scores: { reliability: 80, output_quality: 60 },
      findings: [{
        id: 'f1',
        severity: 'medium',
        category: 'output_quality',
        title: 'Tighten the output',
        why: 'The result is verbose.',
        impact: 'Readers spend longer scanning it.',
        suggested_change: 'Make the summary shorter.',
      }],
      proposal_validated: true,
      proposal_summary: 'Use a shorter transformation.',
      proposed_workflow: {
        name: 'Reviewed flow',
        nodes: [{ id: 'manual_1', type: 'manualTrigger', name: 'Manual', params: {} }],
        edges: [],
      },
    });
    const updateSpy = vi.spyOn(api, 'updateWorkflow').mockResolvedValue({});
    const triggerSpy = vi.spyOn(api, 'triggerWorkflow').mockResolvedValue({});
    let applied = null;

    const { root } = await mountWithApp(AIAssistantDrawer, {
      props: {
        visible: true,
        currentNodes: [{
          id: 'manual_1',
          type: 'custom',
          position: { x: 100, y: 100 },
          data: { id: 'manual_1', type: 'manualTrigger', name: 'Manual', params: {} },
        }],
        currentEdges: [],
        onLoadWorkflow: (workflow) => { applied = workflow; },
      },
    });
    const workflowStore = useWorkflowStore();
    const executionStore = useExecutionStore();
    workflowStore.currentWorkflow = { id: 'wf-1', name: 'Review me' };
    executionStore.executionLogs = [{ id: 'exec-1', status: 'SUCCESS', node_logs: [] }];
    await nextFrame();

    findButton(root, 'Review Latest Run').click();
    await nextFrame();
    const reviewButton = findButton(root, 'Review');
    expect(reviewButton.disabled).toBe(false);
    reviewButton.click();
    await nextFrame();
    await nextFrame();

    expect(reviewSpy).toHaveBeenCalledTimes(1);
    expect(reviewSpy.mock.calls[0][0]).toBe('latest_run');
    expect(reviewSpy.mock.calls[0][3]).toMatchObject({ id: 'exec-1', status: 'SUCCESS' });
    expect(updateSpy).not.toHaveBeenCalled();
    expect(triggerSpy).not.toHaveBeenCalled();

    const applyButton = findButton(root, 'Apply proposal to canvas');
    expect(applyButton).toBeTruthy();
    applyButton.click();
    await nextFrame();

    expect(applied).toMatchObject({ name: 'Reviewed flow' });
    expect(updateSpy).not.toHaveBeenCalled();
    expect(triggerSpy).not.toHaveBeenCalled();
  });
});
