import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useWorkflowStore } from '../src/stores/workflowStore';
import WorkflowsPage from '../src/pages/WorkflowsPage.vue';
import StateBlock from '../src/components/StateBlock.vue';
import { mountWithApp, nextFrame } from './mount';

describe('page loading, empty, and error states', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders empty workflow state with a next action', async () => {
    const { root } = await mountWithApp(WorkflowsPage, { route: '/workflows' });
    const store = useWorkflowStore();
    store.workflows = [];
    store.loading = false;
    await nextFrame();

    expect(root.textContent).toContain('No workflows yet');
    expect(root.textContent).toContain('Create from template');
  });

  it('renders API error state with retry action', async () => {
    const { root } = await mountWithApp(StateBlock, {
      props: {
        tone: 'danger',
        title: 'Workflow request failed',
        message: 'Network request failed',
        actionLabel: 'Retry',
      },
    });

    expect(root.textContent).toContain('Workflow request failed');
    expect(root.textContent).toContain('Network request failed');
    expect(root.querySelector('button')?.textContent).toContain('Retry');
  });
});
