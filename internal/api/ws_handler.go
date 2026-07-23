package api

import (
	"log"
	"net/http"
	"net/url"

	"goflow/internal/engine"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		// Cho phép localhost, 127.0.0.1 hoặc trùng khớp Host của HTTP Request (cùng nguồn)
		if u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Host == r.Host {
			return true
		}
		return false
	},
}

type WSHandler struct {
	eventBus *engine.EventBus
	apiKey   string
}

func NewWSHandler(eb *engine.EventBus, apiKey string) *WSHandler {
	return &WSHandler{eventBus: eb, apiKey: apiKey}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !requireAPIKey(w, r, h.apiKey) {
		return
	}

	responseHeader := http.Header{}
	for _, protocol := range websocketProtocols(r) {
		if len(protocol) > len("goflow.") && protocol[:len("goflow.")] == "goflow." {
			responseHeader.Set("Sec-WebSocket-Protocol", protocol)
			break
		}
	}

	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket: %v", err)
		return
	}
	defer conn.Close()

	ch := h.eventBus.Subscribe()
	defer h.eventBus.Unsubscribe(ch)

	done := make(chan struct{})

	// Keep-alive/Read pump để phát hiện disconnect
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()

	// Broadcast execution events
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
