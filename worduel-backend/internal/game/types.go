package game

import "time"

// Player represents one participant in a match.
type Player struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Rating   int    `json:"rating"`
}

// Move is a single word submission made by a player during a match.
type Move struct {
	PlayerID    string    `json:"player_id"`
	Word        string    `json:"word"`
	Valid       bool      `json:"valid"`
	Score       int       `json:"score"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// MatchResult is the final, immutable summary of a finished match — this is
// the shape that gets flushed to PostgreSQL's `matches` + `moves` tables
// when a session goroutine exits (see schema.sql).
type MatchResult struct {
	MatchID   string    `json:"match_id"`
	Player1   Player    `json:"player1"`
	Player2   Player    `json:"player2"`
	Scores    map[string]int `json:"scores"`
	Moves     []Move    `json:"moves"`
	WinnerID  string    `json:"winner_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// letterScore is a Scrabble-style per-letter point value, used to score
// submitted words. Kept simple and self-contained.
var letterScore = map[rune]int{
	'A': 1, 'E': 1, 'I': 1, 'O': 1, 'U': 1, 'L': 1, 'N': 1, 'S': 1, 'T': 1, 'R': 1,
	'D': 2, 'G': 2,
	'B': 3, 'C': 3, 'M': 3, 'P': 3,
	'F': 4, 'H': 4, 'V': 4, 'W': 4, 'Y': 4,
	'K': 5,
	'J': 8, 'X': 8,
	'Q': 10, 'Z': 10,
}

// ScoreWord computes a word's point value.
func ScoreWord(word string) int {
	total := 0
	for _, r := range word {
		total += letterScore[r]
	}
	return total
}
