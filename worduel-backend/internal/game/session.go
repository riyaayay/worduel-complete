package game

import (
	"math/rand"
	"strings"
	"time"
)

// Validator is satisfied by *trie.Trie. Defined here (not imported from the
// trie package) so game stays decoupled and independently testable with a
// fake dictionary.
type Validator interface {
	IsValidWord(word string) bool
}

// Broadcaster decouples the game package from the websocket hub. The hub
// implements this; a session only ever talks to matches through it.
type Broadcaster interface {
	Broadcast(matchID string, v interface{})
	SendTo(playerID string, v interface{})
}

// MoveRequest is what arrives on a session's move channel from the hub when
// a player submits a word over the websocket.
type MoveRequest struct {
	PlayerID string
	Word     string
}

const (
	boardSize = 16
	// MatchDuration is exported so other packages (e.g. ws, for a
	// reconnect sync message) can reference the same value.
	MatchDuration = 90 * time.Second
	idleTimeout   = 60 * time.Second
)

// matchDuration keeps the shorter internal name used throughout this file.
const matchDuration = MatchDuration

var vowels = []rune("AEIOU")
var consonants = []rune("BCDFGHJKLMNPQRSTVWXYZ")

// Session is one active match. Exactly one goroutine (Run) ever touches its
// mutable fields, so there is no shared-memory race to guard with a mutex —
// the channel is the synchronization primitive, which is the idiomatic Go
// answer to "how do you avoid locks here".
type Session struct {
	ID      string
	P1, P2  Player
	board   []rune
	scores  map[string]int
	moves   []Move
	found   map[string]bool // "playerID:WORD" -> already scored, prevents re-submission farming

	MoveCh chan MoveRequest
	queryCh chan chan StateSnapshot
	done   chan struct{}

	validator   Validator
	broadcaster Broadcaster
	onFinish    func(MatchResult)
	startedAt   time.Time
}

// NewSession builds a fresh match between two players. It generates a
// pseudo-random letter board (roughly 40% vowels, like a Boggle tray) that
// both players draw words from.
func NewSession(id string, p1, p2 Player, validator Validator, broadcaster Broadcaster, onFinish func(MatchResult)) *Session {
	return &Session{
		ID:          id,
		P1:          p1,
		P2:          p2,
		board:       generateBoard(),
		scores:      map[string]int{p1.ID: 0, p2.ID: 0},
		found:       make(map[string]bool),
		MoveCh:      make(chan MoveRequest, 32),
		queryCh:     make(chan chan StateSnapshot),
		done:        make(chan struct{}),
		validator:   validator,
		broadcaster: broadcaster,
		onFinish:    onFinish,
		startedAt:   time.Now(),
	}
}

func generateBoard() []rune {
	n := boardSize * 4 / 10
	board := make([]rune, 0, boardSize)
	for i := 0; i < n; i++ {
		board = append(board, vowels[rand.Intn(len(vowels))])
	}
	for i := n; i < boardSize; i++ {
		board = append(board, consonants[rand.Intn(len(consonants))])
	}
	rand.Shuffle(len(board), func(i, j int) { board[i], board[j] = board[j], board[i] })
	return board
}

// StateSnapshot is a point-in-time, read-only copy of a session's state,
// used to bring a freshly-connected websocket client up to speed (e.g. a
// player who connects a few hundred ms after the match was created and
// missed the original "match_start" broadcast).
type StateSnapshot struct {
	MatchID string        `json:"match_id"`
	Board   []string      `json:"board"`
	Players []Player      `json:"players"`
	Scores  map[string]int `json:"scores"`
}

// State requests a snapshot from the owning goroutine via a request/response
// channel round-trip — the idiomatic Go alternative to locking a mutex
// around the session's fields for this one-off read from another
// goroutine (the hub, when a client connects).
func (s *Session) State() StateSnapshot {
	resp := make(chan StateSnapshot)
	s.queryCh <- resp
	return <-resp
}

func (s *Session) snapshot() StateSnapshot {
	scoresCopy := make(map[string]int, len(s.scores))
	for k, v := range s.scores {
		scoresCopy[k] = v
	}
	return StateSnapshot{
		MatchID: s.ID,
		Board:   s.Board(),
		Players: []Player{s.P1, s.P2},
		Scores:  scoresCopy,
	}
}

// Board returns the letter tiles as a string slice, safe to call before Run
// starts (used once to send the initial "match_found" payload).
func (s *Session) Board() []string {
	out := make([]string, len(s.board))
	for i, r := range s.board {
		out[i] = string(r)
	}
	return out
}

// Run is the session's owning goroutine. It processes moves sequentially
// off MoveCh, so two simultaneous submissions from the two players can
// never race on session state.
func (s *Session) Run() {
	matchTimer := time.NewTimer(matchDuration)
	idleTimer := time.NewTimer(idleTimeout)
	defer matchTimer.Stop()
	defer idleTimer.Stop()

	s.broadcaster.Broadcast(s.ID, map[string]interface{}{
		"type":     "match_start",
		"match_id": s.ID,
		"board":    s.Board(),
		"players":  []Player{s.P1, s.P2},
		"duration_seconds": int(matchDuration.Seconds()),
	})

	for {
		select {
		case req := <-s.MoveCh:
			s.handleMove(req)
			if !idleTimer.Stop() {
				<-idleTimer.C
			}
			idleTimer.Reset(idleTimeout)

		case respCh := <-s.queryCh:
			respCh <- s.snapshot()

		case <-matchTimer.C:
			s.finish()
			return

		case <-idleTimer.C:
			s.finish()
			return

		case <-s.done:
			s.finish()
			return
		}
	}
}

// Stop allows an external caller (e.g. both sockets disconnected) to end
// the match early.
func (s *Session) Stop() {
	select {
	case s.done <- struct{}{}:
	default:
	}
}

func (s *Session) handleMove(req MoveRequest) {
	word := strings.ToUpper(strings.TrimSpace(req.Word))
	key := req.PlayerID + ":" + word

	valid := len(word) >= 3 &&
		canFormWord(s.board, word) &&
		s.validator.IsValidWord(word) &&
		!s.found[key]

	points := 0
	if valid {
		points = ScoreWord(word)
		s.scores[req.PlayerID] += points
		s.found[key] = true
	}

	mv := Move{
		PlayerID:    req.PlayerID,
		Word:        word,
		Valid:       valid,
		Score:       points,
		SubmittedAt: time.Now(),
	}
	s.moves = append(s.moves, mv)

	// Broadcast the outcome and both live scores to both players so the
	// opponent's score updates in real time on the live match screen.
	s.broadcaster.Broadcast(s.ID, map[string]interface{}{
		"type":   "move_result",
		"move":   mv,
		"scores": s.scores,
	})
}

func (s *Session) finish() {
	winner := s.P1.ID
	if s.scores[s.P2.ID] > s.scores[s.P1.ID] {
		winner = s.P2.ID
	} else if s.scores[s.P2.ID] == s.scores[s.P1.ID] {
		winner = "" // tie
	}

	result := MatchResult{
		MatchID:   s.ID,
		Player1:   s.P1,
		Player2:   s.P2,
		Scores:    s.scores,
		Moves:     s.moves,
		WinnerID:  winner,
		StartedAt: s.startedAt,
		EndedAt:   time.Now(),
	}

	s.broadcaster.Broadcast(s.ID, map[string]interface{}{
		"type":   "match_end",
		"result": result,
	})

	if s.onFinish != nil {
		s.onFinish(result)
	}
}

// canFormWord checks that `word` can be built from the multiset of letters
// on `board` (each tile usable at most once per word).
func canFormWord(board []rune, word string) bool {
	counts := make(map[rune]int, len(board))
	for _, r := range board {
		counts[r]++
	}
	for _, r := range word {
		if counts[r] <= 0 {
			return false
		}
		counts[r]--
	}
	return true
}
