package api

import (
	"net/http"

	"goflow/internal/nodes"
)

type NodeHandler struct {
	registry *nodes.PluginRegistry
}

func NewNodeHandler(r *nodes.PluginRegistry) *NodeHandler {
	return &NodeHandler{registry: r}
}

func (h *NodeHandler) ListDefinitions(w http.ResponseWriter, r *http.Request) {
	defs := h.registry.ListDefinitions()
	renderJSON(w, http.StatusOK, defs)
}
