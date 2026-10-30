// Package api exposes WordDuel's REST surface (matchmaking + leaderboard +
// hints) and the WebSocket upgrade endpoint, wiring together the
// matchmaking queue, the game sessions it spawns, the websocket hub, the
// dictionary trie, and the store.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"worduel-backend/internal/game"
	"worduel-backend/internal/matchmaking"
	"worduel-backend/internal/store"
	"worduel-backend/internal/trie"
	"worduel-backend/internal/ws"
)

// App holds every wired-up dependency and implements http.Handler methods
// for each route.
type App struct {
	Hub   *ws.Hub
	Queue *matchmaking.Queue
	Dict  *trie.Trie
	Store store.Store

	mu       sync.RWMutex
	assigned map[string]string // playerID -> matchID, populated as pairs form
}

// NewApp wires a fresh App. The matchmaking queue is set separately via
// SetQueue once it exists, since building the queue itself requires a
// session factory that closes over this App (see cmd/server/main.go).
func NewApp(hub *ws.Hub, dict *trie.Trie, st store.Store) *App {
	return &App{Hub: hub, Dict: dict, Store: st, assigned: make(map[string]string)}
}

// SetQueue attaches the matchmaking queue and starts the background
// goroutine that drains newly formed matches off it.
func (a *App) SetQueue(queue *matchmaking.Queue) {
	a.Queue = queue
	go a.trackMatches()
}

func (a *App) trackMatches() {
	for session := range a.Queue.Matches() {
		a.Hub.RegisterSession(session)
		a.mu.Lock()
		a.assigned[session.P1.ID] = session.ID
		a.assigned[session.P2.ID] = session.ID
		a.mu.Unlock()
		// Registration and player-to-match assignment must happen before Run;
		// otherwise a fast client can receive an "unknown match" websocket 404.
		go session.Run()
	}
}

func randomID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// NewSessionFactory returns a matchmaking.SessionFactory closed over this
// App's dictionary, hub, and store — used once at startup to build the
// matchmaking.Queue.
func (a *App) NewSessionFactory() matchmaking.SessionFactory {
	return func(p1, p2 game.Player) *game.Session {
		matchID := randomID("match")
		return game.NewSession(matchID, p1, p2, a.Dict, a.Hub, func(result game.MatchResult) {
			if err := a.Store.SaveMatchResult(result); err != nil {
				log.Printf("persist match %s: %v", result.MatchID, err)
			}
			a.mu.Lock()
			delete(a.assigned, result.Player1.ID)
			delete(a.assigned, result.Player2.ID)
			a.mu.Unlock()
			a.Hub.CleanupMatch(result.MatchID)
		})
	}
}

// --- HTTP handlers -------------------------------------------------------

type queueRequest struct {
	Username string `json:"username"`
}

// HandleQueue: POST /api/queue — registers (or re-uses) a player and drops
// them into the matchmaking channel.
func (a *App) HandleQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req queueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || len(req.Username) > 32 {
		http.Error(w, "username must be 1-32 characters", http.StatusBadRequest)
		return
	}

	player := game.Player{ID: randomID("player"), Username: req.Username, Rating: 1000}
	if _, err := a.Store.SaveUser(player); err != nil {
		http.Error(w, "unable to save player", http.StatusServiceUnavailable)
		return
	}
	if !a.Queue.Enqueue(matchmaking.Ticket{Player: player}) {
		http.Error(w, "matchmaking queue is full; please retry", http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, map[string]interface{}{
		"player_id": player.ID,
		"username":  player.Username,
		"status":    "queued",
	})
}

// HandleQueueCancel: DELETE /api/queue?player_id=X — lets a client that
// gives up waiting remove its ticket rather than becoming a ghost opponent.
func (a *App) HandleQueueCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	playerID := r.URL.Query().Get("player_id")
	if playerID == "" {
		http.Error(w, "player_id required", http.StatusBadRequest)
		return
	}
	a.Queue.Cancel(playerID)
	writeJSON(w, map[string]string{"status": "cancelled"})
}

// HandleQueueStatus: GET /api/queue/status?player_id=X — the client polls
// this (e.g. every 1s) until a match_id appears, then opens the websocket.
func (a *App) HandleQueueStatus(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("player_id")
	if playerID == "" {
		http.Error(w, "player_id required", http.StatusBadRequest)
		return
	}
	a.mu.RLock()
	matchID, ok := a.assigned[playerID]
	a.mu.RUnlock()

	if !ok {
		writeJSON(w, map[string]interface{}{"status": "waiting"})
		return
	}
	writeJSON(w, map[string]interface{}{"status": "matched", "match_id": matchID})
}

// HandleWS: GET /ws?player_id=X&match_id=Y — upgrades to a websocket
// connection attached to the given match.
func (a *App) HandleWS(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("player_id")
	matchID := r.URL.Query().Get("match_id")
	if playerID == "" || matchID == "" {
		http.Error(w, "player_id and match_id required", http.StatusBadRequest)
		return
	}
	a.Hub.ServeWS(w, r, matchID, playerID)
}

// HandleLeaderboard: GET /api/leaderboard
func (a *App) HandleLeaderboard(w http.ResponseWriter, r *http.Request) {
	entries, err := a.Store.Leaderboard(20)
	if err != nil {
		http.Error(w, "unable to load leaderboard", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, entries)
}

// HandleHint: GET /api/hint?prefix=CA — the trie's WordsWithPrefix DFS,
// exposed as a lightweight "hint" endpoint.
func (a *App) HandleHint(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	if len(prefix) < 2 {
		http.Error(w, "prefix must be at least 2 characters", http.StatusBadRequest)
		return
	}
	words := a.Dict.WordsWithPrefix(prefix)
	if len(words) > 10 {
		words = words[:10]
	}
	writeJSON(w, map[string]interface{}{"prefix": prefix, "words": words})
}

// HandleHealth: GET /healthz
func (a *App) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{"status": "ok", "dictionary_size": a.Dict.Size()})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
