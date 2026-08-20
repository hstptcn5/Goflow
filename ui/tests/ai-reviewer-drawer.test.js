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
  it('uses latest execution, shows Vietnamese controls, applies explicit param deletions and never saves or runs automatically', async () => {
    const credential = {
      id: 'deepseek-reviewer',
      name: 'DeepSeek Reviewer',
      type: 'API_KEY',
      kind: 'API_KEY',
      provider: 'deepseek',
    };
    vi.spyOn(api, 'getCredentials').mockResolvedValue([credential]);
    const reviewSpy = vi.spyOn(api, 'reviewAIWorkflow').mockResolvedValue({
      mode: 'latest_run',
      provider: 'deepseek',
      model: 'deepseek-v4-flash',
      summary: 'Kết quả gần nhất có thể được cải thiện.',
      scores: { reliability: 80, output_quality: 60 },
      findings: [{
        id: 'f1',
        severity: 'medium',
        category: 'output_quality',
        title: 'Rút gọn đầu ra',
        why: 'Kết quả còn dài.',
        impact: 'Người đọc mất nhiều thời gian hơn.',
        suggested_change: 'Rút ngắn phần tóm tắt.',
        evidence: 'execution exec-1',
        confidence: 90,
      }],
      proposal_validated: true,
      proposal_summary: 'Giữ workflow nhưng loại cấu hình nhạy cảm cũ.',
      proposal_delete_params: { sheet_1: ['service_account_json'] },
      proposed_workflow: {
        name: 'Reviewed flow',
        nodes: [{
          id: 'sheet_1',
          type: 'googleSheets',
          name: 'Google Sheets',
          params: { spreadsheet_id: 'sheet-id', sheet_name: 'Sheet1', action: 'APPEND' },
        }],
        edges: [],
      },
    });
    const updateSpy = vi.spyOn(api, 'updateWorkflow').mockResolvedValue({});
    const triggerSpy = vi.spyOn(api, 'triggerWorkflow').mockResolvedValue({});
    let applied = null;
    const currentNodes = [{
      id: 'sheet_1',
      type: 'custom',
      position: { x: 100, y: 100 },
      data: {
        id: 'sheet_1',
        type: 'googleSheets',
        name: 'Google Sheets',
        params: { service_account_json: 'legacy-inline-config', spreadsheet_id: 'sheet-id' },
      },
    }];

    const { root } = await mountWithApp(AIAssistantDrawer, {
      props: {
        visible: true,
        currentNodes,
        currentEdges: [],
        onLoadWorkflow: (workflow) => { applied = workflow; },
      },
    });
    const workflowStore = useWorkflowStore();
    const executionStore = useExecutionStore();
    workflowStore.currentWorkflow = { id: 'wf-1', name: 'Review me' };
    executionStore.executionLogs = [{ id: 'exec-1', status: 'SUCCESS', node_logs: [] }];
    await nextFrame();

    findButton(root, 'Đánh giá lần chạy gần nhất').click();
    await nextFrame();
    const reviewButton = root.querySelector('.btn-send');
    expect(reviewButton.textContent.trim()).toBe('Đánh giá');
    expect(reviewButton.disabled).toBe(false);
    reviewButton.click();
    await nextFrame();
    await nextFrame();

    expect(reviewSpy).toHaveBeenCalledTimes(1);
    expect(reviewSpy.mock.calls[0][0]).toBe('latest_run');
    expect(reviewSpy.mock.calls[0][3]).toMatchObject({ id: 'exec-1', status: 'SUCCESS' });
    expect(root.textContent).toContain('Bằng chứng:');
    expect(root.textContent).toContain('Độ tin cậy:');
    expect(updateSpy).not.toHaveBeenCalled();
    expect(triggerSpy).not.toHaveBeenCalled();

    const applyButton = findButton(root, 'Áp dụng đề xuất lên canvas');
    expect(applyButton).toBeTruthy();
    applyButton.click();
    await nextFrame();

    expect(currentNodes[0].data.params.service_account_json).toBeUndefined();
    expect(applied).toMatchObject({ name: 'Reviewed flow' });
    expect(updateSpy).not.toHaveBeenCalled();
    expect(triggerSpy).not.toHaveBeenCalled();
  });

  it('renders unavailable workflow-only scores as N/A instead of zero', async () => {
    vi.spyOn(api, 'getCredentials').mockResolvedValue([{
      id: 'deepseek-reviewer', name: 'DeepSeek Reviewer', type: 'API_KEY', kind: 'API_KEY', provider: 'deepseek',
    }]);
    vi.spyOn(api, 'reviewAIWorkflow').mockResolvedValue({
      mode: 'workflow',
      provider: 'deepseek',
      model: 'deepseek-v4-flash',
      summary: 'Workflow hợp lệ.',
      scores: { reliability: 90, output_quality: null },
      findings: [],
      proposal_validated: false,
    });

    const { root } = await mountWithApp(AIAssistantDrawer, {
      props: {
        visible: true,
        currentNodes: [{ id: 'manual_1', data: { type: 'manualTrigger', name: 'Manual', params: {} }, position: { x: 0, y: 0 } }],
        currentEdges: [],
      },
    });
    await nextFrame();
    findButton(root, 'Đánh giá workflow').click();
    await nextFrame();
    root.querySelector('.btn-send').click();
    await nextFrame();
    await nextFrame();

    const outputScore = Array.from(root.querySelectorAll('.score-item')).find((item) => item.textContent.includes('Chất lượng đầu ra'));
    expect(outputScore?.textContent).toContain('N/A');
  });
});
