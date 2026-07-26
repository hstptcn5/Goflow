package api

import (
	"net/http"
	"strconv"

	"goflow/internal/storage"
)

type AuditHandler struct {
	auditStore *storage.AuditStore
}

func NewAuditHandler(auditStore *storage.AuditStore) *AuditHandler {
	return &AuditHandler{auditStore: auditStore}
}

func (h *AuditHandler) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := h.auditStore.List(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if events == nil {
		events = []storage.AuditEvent{}
	}
	renderJSON(w, http.StatusOK, events)
}
