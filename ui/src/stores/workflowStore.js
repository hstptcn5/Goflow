import { defineStore } from 'pinia';
import { api } from '@/services/api';

export const useWorkflowStore = defineStore('workflow', {
  state: () => ({
    workflows: [],
    currentWorkflow: null,
    nodeDefinitions: [],
    credentials: [],
    selectedNodeId: null,
    isDirty: false,
    loading: false,
    error: null,
  }),

  getters: {
    selectedNode: (state) => {
      if (!state.currentWorkflow || !state.selectedNodeId) return null;
      try {
        const nodes = typeof state.currentWorkflow.nodes_json === 'string'
          ? JSON.parse(state.currentWorkflow.nodes_json)
          : state.currentWorkflow.nodes_json;
        return nodes.find((n) => n.id === state.selectedNodeId) || null;
      } catch {
        return null;
      }
    },
  },

  actions: {
    async fetchWorkflows() {
      this.loading = true;
      try {
        this.workflows = await api.getWorkflows();
      } catch (err) {
        this.error = err.message;
      } finally {
        this.loading = false;
      }
    },

    async fetchNodeDefinitions() {
      try {
        this.nodeDefinitions = await api.getNodeDefinitions();
      } catch (err) {
        console.error('Failed to fetch node defs', err);
      }
    },

    async fetchCredentials() {
      try {
        this.credentials = await api.getCredentials();
      } catch (err) {
        console.error('Failed to fetch credentials', err);
      }
    },

    async selectWorkflow(id) {
      this.loading = true;
      try {
        this.currentWorkflow = await api.getWorkflow(id);
        this.selectedNodeId = null;
        this.isDirty = false;
      } catch (err) {
        this.error = err.message;
      } finally {
        this.loading = false;
      }
    },

    async createWorkflow(name, description) {
      try {
        const newWf = await api.createWorkflow({
          name,
          description,
          nodes_json: '[]',
          edges_json: '[]',
        });
        this.workflows.unshift(newWf);
        this.currentWorkflow = newWf;
        return newWf;
      } catch (err) {
        this.error = err.message;
        throw err;
      }
    },

    async saveCurrentWorkflow(nodes, edges) {
      if (!this.currentWorkflow) return;
      try {
        const payload = {
          name: this.currentWorkflow.name,
          description: this.currentWorkflow.description,
          is_active: this.currentWorkflow.is_active,
          nodes_json: JSON.stringify(nodes),
          edges_json: JSON.stringify(edges),
        };
        const updated = await api.updateWorkflow(this.currentWorkflow.id, payload);
        this.currentWorkflow = updated;
        this.isDirty = false;
      } catch (err) {
        this.error = err.message;
        throw err;
      }
    },

    async toggleActive(id, isActive) {
      try {
        await api.toggleWorkflowActive(id, isActive);
        const wf = this.workflows.find((w) => w.id === id);
        if (wf) wf.is_active = isActive;
        if (this.currentWorkflow && this.currentWorkflow.id === id) {
          this.currentWorkflow.is_active = isActive;
        }
      } catch (err) {
        this.error = err.message;
      }
    },

    async deleteWorkflow(id) {
      try {
        await api.deleteWorkflow(id);
        this.workflows = this.workflows.filter((w) => w.id !== id);
        if (this.currentWorkflow && this.currentWorkflow.id === id) {
          this.currentWorkflow = null;
        }
      } catch (err) {
        this.error = err.message;
      }
    },
  },
});
