package api

import (
	"log"
	"net/http"

	"goflow/internal/engine"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Cho phép kết nối từ mọi origin trong môi trường local
	},
}

type WSHandler struct {
	eventBus *engine.EventBus
}

func NewWSHandler(eb *engine.EventBus) *WSHandler {
	return &WSHandler{eventBus: eb}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket: %v", err)
		return
	}
	defer conn.Close()

	ch := h.eventBus.Subscribe()
	defer h.eventBus.Unsubscribe(ch)

	// Keep-alive/Read pump để phát hiện disconnect
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()

	// Broadcast execution events
	for event := range ch {
		if err := conn.WriteJSON(event); err != nil {
			break
		}
	}
}
