import { describe, expect, it } from 'vitest';
import WorkflowEditor from '../src/components/WorkflowEditor.vue';
import { useWorkflowStore } from '../src/stores/workflowStore';
import { mountWithApp, nextFrame } from './mount';

describe('WorkflowEditor top bar', () => {
  it('shows save state and one primary action', async () => {
    const { root } = await mountWithApp(WorkflowEditor, { route: '/workflows/wf-1' });
    const store = useWorkflowStore();
    store.currentWorkflow = {
      id: 'wf-1',
      name: 'Editor Test',
      description: '',
      is_active: true,
      nodes_json: '[]',
      edges_json: '[]',
    };
    store.markDirty();
    await nextFrame();

    expect(root.textContent).toContain('Unsaved changes');
    const primaryButtons = Array.from(root.querySelectorAll('.workflow-topbar .btn-primary'));
    expect(primaryButtons).toHaveLength(1);
    expect(primaryButtons[0].textContent).toContain('Test Workflow');
  });
});
