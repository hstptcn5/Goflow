import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

const apiMocks = vi.hoisted(() => ({
  updateWorkflow: vi.fn(),
}));

vi.mock('@/services/api', () => ({
  api: apiMocks,
}));

describe('workflow dirty and save state', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiMocks.updateWorkflow.mockReset();
  });

  it('marks workflow as dirty when edited', async () => {
    const { useWorkflowStore } = await import('../src/stores/workflowStore');
    const store = useWorkflowStore();

    store.markDirty();

    expect(store.isDirty).toBe(true);
    expect(store.saveState).toBe('dirty');
  });

  it('returns to saved after save succeeds', async () => {
    const { useWorkflowStore } = await import('../src/stores/workflowStore');
    const store = useWorkflowStore();
    store.currentWorkflow = {
      id: 'wf-1',
      name: 'Demo',
      description: '',
      is_active: true,
    };
    store.markDirty();
    apiMocks.updateWorkflow.mockResolvedValue({
      id: 'wf-1',
      name: 'Demo',
      nodes_json: '[]',
      edges_json: '[]',
    });

    await store.saveCurrentWorkflow([], []);

    expect(store.isDirty).toBe(false);
    expect(store.saveState).toBe('saved');
    expect(store.saveError).toBe('');
  });

  it('keeps a visible failed state after save fails', async () => {
    const { useWorkflowStore } = await import('../src/stores/workflowStore');
    const store = useWorkflowStore();
    store.currentWorkflow = {
      id: 'wf-1',
      name: 'Demo',
      description: '',
      is_active: true,
    };
    apiMocks.updateWorkflow.mockRejectedValue(new Error('network down'));

    await expect(store.saveCurrentWorkflow([], [])).rejects.toThrow('network down');

    expect(store.saveState).toBe('failed');
    expect(store.saveError).toBe('network down');
  });
});
