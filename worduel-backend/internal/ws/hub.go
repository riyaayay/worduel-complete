// Package ws implements the WebSocket hub: it registers connected clients,
// routes their incoming move messages to the right game session, and
// broadcasts session events (opponent moves, score updates, match end)
// back out in real time.
package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"worduel-backend/internal/game"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Demo-friendly CORS: accept any origin. Tighten this before shipping.
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	playerID string
	matchID  string
	conn     *websocket.Conn
	send     chan interface{}
}

type matchEntry struct {
	session *game.Session
	clients map[string]*client // playerID -> client
}

// Hub owns every live connection and every active session's client set.
type Hub struct {
	mu      sync.RWMutex
	matches map[string]*matchEntry
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{matches: make(map[string]*matchEntry)}
}

// RegisterSession makes the hub aware of a newly created match before any
// client has connected to it, so the first "match_start" broadcast has
// somewhere to land once players do connect.
func (h *Hub) RegisterSession(s *game.Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.matches[s.ID] = &matchEntry{session: s, clients: make(map[string]*client)}
}

// ServeWS upgrades an HTTP request to a WebSocket connection and attaches
// it to the given match/player.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, matchID, playerID string) {
	h.mu.RLock()
	entry, ok := h.matches[matchID]
	h.mu.RUnlock()
	if !ok {
		http.Error(w, "unknown match", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	c := &client{playerID: playerID, matchID: matchID, conn: conn, send: make(chan interface{}, 16)}

	h.mu.Lock()
	entry.clients[playerID] = c
	h.mu.Unlock()

	go h.writePump(c)

	// A player can connect slightly after the match's owning goroutine
	// already broadcast "match_start" (they were still polling
	// /api/queue/status), so bring them up to date directly rather than
	// relying on having caught the original broadcast.
	snap := entry.session.State()
	c.send <- map[string]interface{}{
		"type":              "match_start",
		"match_id":          snap.MatchID,
		"board":             snap.Board,
		"players":           snap.Players,
		"scores":            snap.Scores,
		"duration_seconds":  int(game.MatchDuration.Seconds()),
	}

	h.readPump(entry, c)
}

func (h *Hub) readPump(entry *matchEntry, c *client) {
	defer func() {
		c.conn.Close()
		h.mu.Lock()
		delete(entry.clients, c.playerID)
		remaining := len(entry.clients)
		h.mu.Unlock()
		if remaining == 0 {
			entry.session.Stop()
		}
	}()

	c.conn.SetReadLimit(1024)
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg struct {
			Type string `json:"type"`
			Word string `json:"word"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "move":
			select {
			case entry.session.MoveCh <- game.MoveRequest{PlayerID: c.playerID, Word: msg.Word}:
			default:
				log.Printf("session %s move channel full, dropping move", entry.session.ID)
			}
		}
	}
}

func (h *Hub) writePump(c *client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case v, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(v); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Broadcast implements game.Broadcaster: send v to every client currently
// connected to matchID. Non-blocking per-client so one slow reader can't
// stall the match's owning goroutine.
func (h *Hub) Broadcast(matchID string, v interface{}) {
	h.mu.RLock()
	entry, ok := h.matches[matchID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range entry.clients {
		select {
		case c.send <- v:
		default:
			log.Printf("client %s send buffer full, dropping broadcast", c.playerID)
		}
	}
}

// SendTo implements game.Broadcaster: send v to a single player, if they
// are connected to any match. (Currently unused directly by Session, kept
// for symmetry / future direct-message features like private hints.)
func (h *Hub) SendTo(playerID string, v interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, entry := range h.matches {
		if c, ok := entry.clients[playerID]; ok {
			select {
			case c.send <- v:
			default:
			}
			return
		}
	}
}

// CleanupMatch removes bookkeeping for a finished match. Call this once a
// session's Run() has returned (e.g. via the onFinish callback in main.go).
func (h *Hub) CleanupMatch(matchID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.matches, matchID)
}
