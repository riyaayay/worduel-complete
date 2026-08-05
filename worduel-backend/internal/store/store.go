// Package store defines the persistence boundary. Store is the interface
// the rest of the app depends on; MemoryStore is a demo-friendly in-memory
// implementation. A PostgresStore implementing the same interface (against
// the schema in schema.sql) is the natural next step and requires no
// changes anywhere else in the codebase.
package store

import (
	"sort"
	"sync"

	"worduel-backend/internal/game"
)

// Store is what the API layer and matchmaking-finish callback depend on.
type Store interface {
	SaveUser(p game.Player) game.Player
	GetUser(id string) (game.Player, bool)
	SaveMatchResult(result game.MatchResult)
	Leaderboard(limit int) []LeaderboardEntry
}

// LeaderboardEntry is one row of the ranked leaderboard. In production this
// maps directly onto a Redis sorted set (ZADD leaderboard {rating} {id},
// ZREVRANGE for reads) — see README for the Redis-backed version.
type LeaderboardEntry struct {
	Player game.Player `json:"player"`
	Rating int         `json:"rating"`
}

// MemoryStore is a mutex-protected in-memory Store. Fine for a demo/single
// instance; swap for Postgres + Redis (see schema.sql) to run this for
// real. Note this is the *only* place in the codebase that uses a mutex for
// shared state — match state itself is owned by a single goroutine per
// session (see internal/game/session.go) and needs none.
type MemoryStore struct {
	mu      sync.RWMutex
	users   map[string]game.Player
	results []game.MatchResult
}

// NewMemoryStore returns an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{users: make(map[string]game.Player)}
}

func (s *MemoryStore) SaveUser(p game.Player) game.Player {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[p.ID] = p
	return p
}

func (s *MemoryStore) GetUser(id string) (game.Player, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.users[id]
	return p, ok
}

func (s *MemoryStore) SaveMatchResult(result game.MatchResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = append(s.results, result)

	// Update ratings: simple +/-10 win/loss adjustment, enough to make the
	// leaderboard move for a demo. A real system would use Elo/Glicko.
	if result.WinnerID == "" {
		return
	}
	loserID := result.Player1.ID
	if result.WinnerID == result.Player1.ID {
		loserID = result.Player2.ID
	}
	if w, ok := s.users[result.WinnerID]; ok {
		w.Rating += 10
		s.users[result.WinnerID] = w
	}
	if l, ok := s.users[loserID]; ok {
		l.Rating -= 5
		s.users[loserID] = l
	}
}

func (s *MemoryStore) Leaderboard(limit int) []LeaderboardEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]LeaderboardEntry, 0, len(s.users))
	for _, u := range s.users {
		entries = append(entries, LeaderboardEntry{Player: u, Rating: u.Rating})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Rating > entries[j].Rating })
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}
