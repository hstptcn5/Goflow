import { defineStore } from 'pinia';
import { api } from '@/services/api';

export const useExecutionStore = defineStore('execution', {
  state: () => ({
    executionLogs: [],
    nodeStatuses: {}, // nodeID -> 'RUNNING' | 'SUCCESS' | 'FAILED'
    nodeEvents: {}, // nodeID -> latest realtime execution event
    currentExecution: null,
    isExecuting: false,
  }),

  actions: {
    async fetchExecutionHistory(workflowId) {
      try {
        this.executionLogs = await api.getExecutions(workflowId);
      } catch (err) {
        console.error('Failed to fetch execution logs', err);
      }
    },

    handleWSEvent(event) {
      if (!event.node_id) return;
      this.nodeStatuses[event.node_id] = event.status;
      this.nodeEvents[event.node_id] = {
        node_id: event.node_id,
        execution_id: event.execution_id,
        workflow_id: event.workflow_id,
        status: event.status,
        duration_ms: event.duration_ms || 0,
        output: event.payload,
        error: event.error,
        timestamp: event.timestamp,
        realtime: true,
      };

      if (event.status === 'RUNNING') {
        this.isExecuting = true;
      } else if (event.status === 'SUCCESS' || event.status === 'FAILED') {
        this.isExecuting = false;
      }
    },

    resetNodeStatuses() {
      this.nodeStatuses = {};
      this.nodeEvents = {};
      this.isExecuting = false;
    },
  },
});
