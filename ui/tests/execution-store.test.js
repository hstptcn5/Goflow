import { describe, expect, it } from 'vitest';
import { useExecutionStore } from '../src/stores/executionStore';
import { mountWithApp } from './mount';

describe('execution store live scoping', () => {
  it('stores live execution events by workflow and execution', async () => {
    await mountWithApp({ template: '<div />' });
    const store = useExecutionStore();

    store.handleWSEvent({
      workflow_id: 'wf_a',
      execution_id: 'exec_a',
      node_id: 'node_1',
      status: 'FAILED',
    });
    store.handleWSEvent({
      workflow_id: 'wf_b',
      execution_id: 'exec_b',
      node_id: 'node_1',
      status: 'SUCCESS',
    });

    expect(store.liveExecutionIdsByWorkflow.wf_a).toBe('exec_a');
    expect(store.liveExecutionIdsByWorkflow.wf_b).toBe('exec_b');
    expect(store.nodeEventsByWorkflow.wf_a.exec_a.node_1.status).toBe('FAILED');
    expect(store.nodeEventsByWorkflow.wf_b.exec_b.node_1.status).toBe('SUCCESS');

    store.resetWorkflowLiveState('wf_a');
    expect(store.liveExecutionIdsByWorkflow.wf_a).toBeUndefined();
    expect(store.liveExecutionIdsByWorkflow.wf_b).toBe('exec_b');
    expect(store.nodeEventsByWorkflow.wf_a).toBeUndefined();
    expect(store.nodeEventsByWorkflow.wf_b.exec_b.node_1.status).toBe('SUCCESS');
  });
});
