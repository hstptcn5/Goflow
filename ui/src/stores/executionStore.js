import { defineStore } from 'pinia';
import { api } from '@/services/api';

export const useExecutionStore = defineStore('execution', {
  state: () => ({
    executionLogs: [],
    nodeStatuses: {}, // nodeID -> 'RUNNING' | 'SUCCESS' | 'FAILED'
    nodeEvents: {}, // nodeID -> latest realtime execution event
    nodeEventsByExecution: {}, // executionID -> nodeID -> realtime execution event
    liveExecutionIdsByWorkflow: {}, // workflowID -> executionID
    nodeEventsByWorkflow: {}, // workflowID -> executionID -> nodeID -> realtime execution event
    liveExecutionId: '',
    currentExecution: null,
    isExecuting: false,
    error: '',
  }),

  actions: {
    async fetchExecutionHistory(workflowId) {
      this.error = '';
      try {
        this.executionLogs = await api.getExecutions(workflowId);
      } catch (err) {
        this.error = err.message;
        throw err;
      }
    },

    handleWSEvent(event) {
      if (!event.node_id) return;
      const normalized = {
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
      if (event.workflow_id && event.execution_id) {
        this.liveExecutionIdsByWorkflow[event.workflow_id] = event.execution_id;
        this.nodeEventsByWorkflow[event.workflow_id] = {
          ...(this.nodeEventsByWorkflow[event.workflow_id] || {}),
          [event.execution_id]: {
            ...(this.nodeEventsByWorkflow[event.workflow_id]?.[event.execution_id] || {}),
            [event.node_id]: normalized,
          },
        };
      }
      if (event.execution_id) {
        this.liveExecutionId = event.execution_id;
        this.nodeEventsByExecution[event.execution_id] = {
          ...(this.nodeEventsByExecution[event.execution_id] || {}),
          [event.node_id]: normalized,
        };
      }
      this.nodeStatuses[event.node_id] = event.status;
      this.nodeEvents[event.node_id] = normalized;

      if (event.status === 'RUNNING') {
        this.isExecuting = true;
      } else if (event.status === 'SUCCESS' || event.status === 'FAILED') {
        this.isExecuting = false;
      }
    },

    resetNodeStatuses() {
      this.nodeStatuses = {};
      this.nodeEvents = {};
      this.nodeEventsByExecution = {};
      this.liveExecutionIdsByWorkflow = {};
      this.nodeEventsByWorkflow = {};
      this.liveExecutionId = '';
      this.isExecuting = false;
    },

    clearExecutionHistory() {
      this.executionLogs = [];
      this.currentExecution = null;
    },

    resetWorkflowLiveState(workflowId) {
      if (!workflowId) return;
      const liveIds = { ...this.liveExecutionIdsByWorkflow };
      delete liveIds[workflowId];
      this.liveExecutionIdsByWorkflow = liveIds;
      const events = { ...this.nodeEventsByWorkflow };
      delete events[workflowId];
      this.nodeEventsByWorkflow = events;
      this.liveExecutionId = '';
      this.nodeStatuses = {};
      this.nodeEvents = {};
    },
  },
});
