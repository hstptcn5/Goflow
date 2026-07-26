import { afterEach, describe, expect, it, vi } from 'vitest';
import WorkflowEditor from '../src/components/WorkflowEditor.vue';
import { useWorkflowStore } from '../src/stores/workflowStore';
import { api } from '../src/services/api';
import { mountWithApp, nextFrame } from './mount';

describe('WorkflowEditor top bar', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

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

  it('adds nodes from picker, quick-add connects, validates, duplicates, and supports undo/redo', async () => {
    const { root, vm } = await mountWithApp(WorkflowEditor, { route: '/workflows/wf-1' });
    const store = useWorkflowStore();
    store.nodeDefinitions = [
      { type: 'manualTrigger', name: 'Manual Trigger', category: 'TRIGGER', description: 'Start manually', params: [] },
      {
        type: 'conditionIf',
        name: 'IF / ELSE Condition',
        category: 'LOGIC',
        description: 'Branch workflow',
        params: [{ name: 'left', label: 'Left value', type: 'text', required: true, default: '' }],
      },
    ];
    store.currentWorkflow = {
      id: 'wf-1',
      name: 'Editor Test',
      description: '',
      is_active: true,
      nodes_json: '[]',
      edges_json: '[]',
    };
    await nextFrame();

    vm.addNodeFromPicker(store.nodeDefinitions[0]);
    await nextFrame();
    expect(vm.nodes).toHaveLength(1);
    expect(store.isDirty).toBe(true);

    vm.openNodePicker({ sourceNodeId: vm.nodes[0].id });
    vm.addNodeFromPicker(store.nodeDefinitions[1]);
    await nextFrame();
    expect(vm.nodes).toHaveLength(2);
    expect(vm.edges).toHaveLength(1);
    expect(root.textContent).toContain('Missing required fields');

    vm.duplicateSelectedNode();
    await nextFrame();
    expect(vm.nodes).toHaveLength(3);
    expect(new Set(vm.nodes.map((node) => node.id)).size).toBe(3);

    vm.undo();
    await nextFrame();
    expect(vm.nodes).toHaveLength(2);
    vm.redo();
    await nextFrame();
    expect(vm.nodes).toHaveLength(3);
  });

  it('keeps edge idle state and can undo auto-layout', async () => {
    const { vm } = await mountWithApp(WorkflowEditor, { route: '/workflows/wf-1' });
    const store = useWorkflowStore();
    store.nodeDefinitions = [
      { type: 'manualTrigger', name: 'Manual Trigger', category: 'TRIGGER', description: 'Start manually', params: [] },
      { type: 'delaySleep', name: 'Delay / Sleep', category: 'LOGIC', description: 'Wait', params: [] },
    ];
    store.currentWorkflow = {
      id: 'wf-1',
      name: 'Editor Test',
      description: '',
      is_active: true,
      nodes_json: JSON.stringify([
        { id: 'b', type: 'delaySleep', name: 'Delay / Sleep', position: { x: 400, y: 400 }, params: {} },
        { id: 'a', type: 'manualTrigger', name: 'Manual Trigger', position: { x: 10, y: 10 }, params: {} },
      ]),
      edges_json: JSON.stringify([{ id: 'e1', source: 'a', target: 'b' }]),
    };
    await nextFrame();

    expect(vm.edges[0].animated).toBe(false);
    const before = vm.nodes.find((node) => node.id === 'b').position.x;
    vm.autoLayout();
    await nextFrame();
    expect(vm.nodes.find((node) => node.id === 'b').position.x).not.toBe(before);
    vm.undo();
    await nextFrame();
    expect(vm.nodes.find((node) => node.id === 'b').position.x).toBe(before);
  });

  it('does not trigger editor shortcuts while typing in a form field', async () => {
    const { root, vm } = await mountWithApp(WorkflowEditor, { route: '/workflows/wf-1' });
    const store = useWorkflowStore();
    store.currentWorkflow = {
      id: 'wf-1',
      name: 'Editor Test',
      description: '',
      is_active: true,
      nodes_json: '[]',
      edges_json: '[]',
    };
    await nextFrame();

    const input = document.createElement('input');
    root.appendChild(input);
    input.focus();
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', bubbles: true }));
    await nextFrame();
    expect(root.textContent).not.toContain('Search by node name');
    expect(vm.nodes).toHaveLength(0);
  });

  it('saves incomplete drafts but blocks Test Workflow until configured', async () => {
    const { root, vm } = await mountWithApp(WorkflowEditor, { route: '/workflows/wf-1' });
    const store = useWorkflowStore();
    store.nodeDefinitions = [
      {
        type: 'conditionIf',
        name: 'IF / ELSE Condition',
        category: 'LOGIC',
        description: 'Branch workflow',
        params: [{ name: 'left', label: 'Input Value', type: 'text', required: true, default: '' }],
      },
    ];
    store.currentWorkflow = {
      id: 'wf-1',
      name: 'Editor Test',
      description: '',
      is_active: true,
      nodes_json: '[]',
      edges_json: '[]',
    };
    vi.spyOn(api, 'updateWorkflow').mockImplementation(async (id, payload) => ({ id, ...payload }));
    const triggerSpy = vi.spyOn(api, 'triggerWorkflow').mockResolvedValue({ id: 'exec-1', status: 'SUCCESS' });
    await nextFrame();

    vm.addNodeFromPicker(store.nodeDefinitions[0]);
    await vm.saveCanvas();
    await nextFrame();
    expect(api.updateWorkflow).toHaveBeenCalled();
    expect(root.textContent).toContain('Saved');

    await vm.runWorkflow();
    await nextFrame();
    expect(triggerSpy).not.toHaveBeenCalled();
    expect(root.textContent).toContain('Fix workflow validation issues before testing.');
  });

  it('does not trigger dirty workflow when save fails', async () => {
    const { root, vm } = await mountWithApp(WorkflowEditor, { route: '/workflows/wf-1' });
    const store = useWorkflowStore();
    store.nodeDefinitions = [
      { type: 'delaySleep', name: 'Delay / Sleep', category: 'LOGIC', description: 'Wait', params: [] },
    ];
    store.currentWorkflow = {
      id: 'wf-1',
      name: 'Editor Test',
      description: '',
      is_active: true,
      nodes_json: '[]',
      edges_json: '[]',
    };
    vi.spyOn(api, 'updateWorkflow').mockRejectedValue(new Error('save unavailable'));
    const triggerSpy = vi.spyOn(api, 'triggerWorkflow').mockResolvedValue({ id: 'exec-1', status: 'SUCCESS' });
    await nextFrame();

    vm.addNodeFromPicker(store.nodeDefinitions[0]);
    await vm.runWorkflow();
    await nextFrame();

    expect(triggerSpy).not.toHaveBeenCalled();
    expect(store.isDirty).toBe(true);
    expect(root.textContent).toContain('save unavailable');
  });

  it('marks Saved when undo restores the saved graph and dirty when redo diverges', async () => {
    const { root, vm } = await mountWithApp(WorkflowEditor, { route: '/workflows/wf-1' });
    const store = useWorkflowStore();
    store.nodeDefinitions = [
      { type: 'manualTrigger', name: 'Manual Trigger', category: 'TRIGGER', description: 'Start', params: [] },
      { type: 'delaySleep', name: 'Delay / Sleep', category: 'LOGIC', description: 'Wait', params: [] },
    ];
    store.currentWorkflow = {
      id: 'wf-1',
      name: 'Editor Test',
      description: '',
      is_active: true,
      nodes_json: JSON.stringify([
        { id: 'a', type: 'manualTrigger', name: 'Manual Trigger', position: { x: 10, y: 10 }, params: {} },
        { id: 'b', type: 'delaySleep', name: 'Delay / Sleep', position: { x: 400, y: 400 }, params: {} },
      ]),
      edges_json: JSON.stringify([{ id: 'e1', source: 'a', target: 'b' }]),
    };
    vi.spyOn(api, 'updateWorkflow').mockImplementation(async (id, payload) => ({ id, ...payload }));
    await nextFrame();

    await vm.saveCanvas();
    await nextFrame();
    expect(root.textContent).toContain('Saved');

    vm.autoLayout();
    await nextFrame();
    expect(root.textContent).toContain('Unsaved changes');
    vm.undo();
    await nextFrame();
    expect(root.textContent).toContain('Saved');
    vm.redo();
    await nextFrame();
    expect(root.textContent).toContain('Unsaved changes');
  });
});
