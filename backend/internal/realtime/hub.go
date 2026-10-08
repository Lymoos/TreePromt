// Package realtime keeps WebSocket clients and tells them when a new revision
// exists. Data never travels over the socket: clients always pull, so a lost
// notification only delays sync, it cannot corrupt it.
package realtime

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

type Client struct {
	UserID uuid.UUID
	Send   chan []byte
}

type Hub struct {
	mu      sync.Mutex
	clients map[uuid.UUID]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[uuid.UUID]map[*Client]struct{})}
}

func (h *Hub) Register(userID uuid.UUID) *Client {
	c := &Client{UserID: userID, Send: make(chan []byte, 16)}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[*Client]struct{})
	}
	h.clients[userID][c] = struct{}{}
	return c
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.clients[c.UserID]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, c.UserID)
		}
	}
}

func (h *Hub) NotifyRevision(userIDs []uuid.UUID, revision int64) {
	msg, _ := json.Marshal(map[string]any{"type": "revision", "revision": revision})
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, uid := range userIDs {
		for c := range h.clients[uid] {
			select {
			case c.Send <- msg:
			default: // клиент не успевает читать — пропускаем, следующее уведомление всё равно приведёт к pull
			}
		}
	}
}

// Count returns the number of connected clients of a user (for tests).
func (h *Hub) Count(userID uuid.UUID) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients[userID])
}
