import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useWorkflowStore } from '../src/stores/workflowStore';
import WorkflowsPage from '../src/pages/WorkflowsPage.vue';
import StateBlock from '../src/components/StateBlock.vue';
import { mountWithApp, nextFrame } from './mount';

describe('page loading, empty, and error states', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders Vietnamese empty workflow state with a next action', async () => {
    const { root } = await mountWithApp(WorkflowsPage, { route: '/workflows' });
    const store = useWorkflowStore();
    store.workflows = [];
    store.loading = false;
    await nextFrame();

    expect(root.textContent).toContain('Chưa có workflow');
    expect(root.textContent).toContain('Tạo từ mẫu');
  });

  it('renders API error state with retry action', async () => {
    const { root } = await mountWithApp(StateBlock, {
      props: {
        tone: 'danger',
        title: 'Không tải được workflow',
        message: 'Không thể kết nối mạng',
        actionLabel: 'Thử lại',
      },
    });

    expect(root.textContent).toContain('Không tải được workflow');
    expect(root.textContent).toContain('Không thể kết nối mạng');
    expect(root.querySelector('button')?.textContent).toContain('Thử lại');
  });
});
