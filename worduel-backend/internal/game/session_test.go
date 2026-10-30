package game

import (
	"sync"
	"testing"
	"time"
)

type testDictionary map[string]bool

func (d testDictionary) IsValidWord(word string) bool { return d[word] }

type capturedBroadcast struct {
	mu     sync.Mutex
	events []map[string]interface{}
}

func (b *capturedBroadcast) Broadcast(_ string, v interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if event, ok := v.(map[string]interface{}); ok {
		b.events = append(b.events, event)
	}
}

func (b *capturedBroadcast) SendTo(_ string, _ interface{}) {}

func (b *capturedBroadcast) count(eventType string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, event := range b.events {
		if event["type"] == eventType {
			count++
		}
	}
	return count
}

func TestSessionScoresValidWordOnlyOnceAndFinishes(t *testing.T) {
	p1 := Player{ID: "p1", Username: "Alice", Rating: 1000}
	p2 := Player{ID: "p2", Username: "Bob", Rating: 1000}
	broadcaster := &capturedBroadcast{}
	resultCh := make(chan MatchResult, 1)
	session := NewSession("match-1", p1, p2, testDictionary{"CAT": true}, broadcaster, func(result MatchResult) {
		resultCh <- result
	})
	session.board = []rune("CATAAAAAAAAAAAAA")

	go session.Run()
	state, active := session.State()
	if !active || len(state.Board) != boardSize {
		t.Fatalf("State() = %#v, %v; want active 16-letter state", state, active)
	}
	session.MoveCh <- MoveRequest{PlayerID: p1.ID, Word: "cat"}
	session.MoveCh <- MoveRequest{PlayerID: p1.ID, Word: "cat"} // duplicate
	session.MoveCh <- MoveRequest{PlayerID: p2.ID, Word: "dog"} // not in dictionary

	deadline := time.After(2 * time.Second)
	for broadcaster.count("move_result") < 3 {
		select {
		case <-deadline:
			t.Fatal("did not receive all move broadcasts")
		case <-time.After(5 * time.Millisecond):
		}
	}
	session.Stop()
	result := <-resultCh
	if got := result.Scores[p1.ID]; got != ScoreWord("CAT") {
		t.Fatalf("p1 score = %d, want %d", got, ScoreWord("CAT"))
	}
	if result.WinnerID != p1.ID {
		t.Fatalf("winner = %q, want %q", result.WinnerID, p1.ID)
	}
	if len(result.Moves) != 3 {
		t.Fatalf("moves = %d, want 3", len(result.Moves))
	}
	if _, active := session.State(); active {
		t.Fatal("finished session reported as active")
	}
}

func TestSessionRejectsPlayerOutsideMatch(t *testing.T) {
	session := NewSession("match-1", Player{ID: "p1"}, Player{ID: "p2"}, testDictionary{}, &capturedBroadcast{}, nil)
	if !session.HasPlayer("p1") || !session.HasPlayer("p2") || session.HasPlayer("intruder") {
		t.Fatal("HasPlayer did not enforce match membership")
	}
}
