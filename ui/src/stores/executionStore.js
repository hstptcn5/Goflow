import { defineStore } from 'pinia';
import { api } from '@/services/api';

export const useExecutionStore = defineStore('execution', {
  state: () => ({
    executionLogs: [],
    nodeStatuses: {}, // nodeID -> 'RUNNING' | 'SUCCESS' | 'FAILED'
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

      if (event.status === 'RUNNING') {
        this.isExecuting = true;
      }
    },

    resetNodeStatuses() {
      this.nodeStatuses = {};
      this.isExecuting = false;
    },
  },
});
