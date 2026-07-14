// Package ws provides a WebSocket hub for broadcasting real-time events
// to connected dashboard clients. Implements the event format from spec §6.3.
package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local usage
	},
}

// Event is the JSON structure broadcast to WebSocket clients.
// Matches spec §6.3 exactly.
type Event struct {
	Type      string `json:"type"`
	Phone     string `json:"phone,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	Error     string `json:"error,omitempty"`
	At        string `json:"at"`
}

// Hub manages WebSocket connections and event broadcasting.
type Hub struct {
	clients    map[*conn]bool
	broadcast  chan Event
	register   chan *conn
	unregister chan *conn
	logger     *slog.Logger
	mu         sync.RWMutex
}

type conn struct {
	ws   *websocket.Conn
	send chan []byte
}

// NewHub creates and starts a new WebSocket hub.
func NewHub(logger *slog.Logger) *Hub {
	h := &Hub{
		clients:    make(map[*conn]bool),
		broadcast:  make(chan Event, 256),
		register:   make(chan *conn),
		unregister: make(chan *conn),
		logger:     logger,
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
			h.logger.Debug("ws client connected", "total", len(h.clients))

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
			h.logger.Debug("ws client disconnected", "total", len(h.clients))

		case evt := <-h.broadcast:
			data, err := json.Marshal(evt)
			if err != nil {
				h.logger.Error("ws marshal event", "error", err)
				continue
			}
			h.mu.Lock()
			for c := range h.clients {
				select {
				case c.send <- data:
				default:
					// Client too slow, disconnect
					close(c.send)
					delete(h.clients, c)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast sends an event to all connected WebSocket clients.
func (h *Hub) Broadcast(evt Event) {
	if evt.At == "" {
		evt.At = time.Now().UTC().Format(time.RFC3339)
	}
	select {
	case h.broadcast <- evt:
	default:
		h.logger.Warn("ws broadcast channel full, dropping event")
	}
}

// HandleWS is the HTTP handler for WebSocket upgrade requests.
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws upgrade failed", "error", err)
		return
	}

	c := &conn{
		ws:   wsConn,
		send: make(chan []byte, 64),
	}

	h.register <- c

	go h.writePump(c)
	go h.readPump(c)
}

func (h *Hub) writePump(c *conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				h.logger.Debug("ws write error", "error", err)
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) readPump(c *conn) {
	defer func() {
		h.unregister <- c
		c.ws.Close()
	}()

	c.ws.SetReadLimit(512)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Debug("ws read error", "error", err)
			}
			return
		}
	}
}

// ClientCount returns the number of connected WebSocket clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
