package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"worduel-backend/internal/game"
)

// PostgresStore is the durable production Store implementation. Active game
// state remains in its session goroutine; this store records users, completed
// matches, moves, and ratings so a restart does not erase the leaderboard.
type PostgresStore struct {
	db *sql.DB
}

func OpenPostgres(ctx context.Context, dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error { return s.db.Close() }

func (s *PostgresStore) SaveUser(p game.Player) (game.Player, error) {
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, rating) VALUES ($1, $2, $3)`,
		p.ID, p.Username, p.Rating,
	)
	if err != nil {
		return game.Player{}, fmt.Errorf("save user: %w", err)
	}
	return p, nil
}

func (s *PostgresStore) GetUser(id string) (game.Player, bool) {
	var p game.Player
	err := s.db.QueryRow(`SELECT id, username, rating FROM users WHERE id = $1`, id).
		Scan(&p.ID, &p.Username, &p.Rating)
	return p, err == nil
}

func (s *PostgresStore) SaveMatchResult(result game.MatchResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin match transaction: %w", err)
	}
	defer tx.Rollback()

	var winner interface{}
	if result.WinnerID != "" {
		winner = result.WinnerID
	}
	board := ""
	for _, letter := range result.Board {
		board += letter
	}
	_, err = tx.Exec(`INSERT INTO matches (id, player1_id, player2_id, winner_id, board, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, result.MatchID, result.Player1.ID, result.Player2.ID,
		winner, board, result.StartedAt, result.EndedAt)
	if err != nil {
		return fmt.Errorf("save match: %w", err)
	}
	for _, move := range result.Moves {
		if _, err := tx.Exec(`INSERT INTO moves (match_id, player_id, word, valid, score, submitted_at)
			VALUES ($1, $2, $3, $4, $5, $6)`, result.MatchID, move.PlayerID, move.Word,
			move.Valid, move.Score, move.SubmittedAt); err != nil {
			return fmt.Errorf("save move: %w", err)
		}
	}
	if result.WinnerID != "" {
		loserID := result.Player1.ID
		if loserID == result.WinnerID {
			loserID = result.Player2.ID
		}
		if _, err := tx.Exec(`UPDATE users SET rating = rating + 10 WHERE id = $1`, result.WinnerID); err != nil {
			return fmt.Errorf("update winner rating: %w", err)
		}
		if _, err := tx.Exec(`UPDATE users SET rating = rating - 5 WHERE id = $1`, loserID); err != nil {
			return fmt.Errorf("update loser rating: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit match transaction: %w", err)
	}
	return nil
}

func (s *PostgresStore) Leaderboard(limit int) ([]LeaderboardEntry, error) {
	rows, err := s.db.Query(`SELECT id, username, rating FROM users ORDER BY rating DESC, username ASC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query leaderboard: %w", err)
	}
	defer rows.Close()
	entries := make([]LeaderboardEntry, 0)
	for rows.Next() {
		var p game.Player
		if err := rows.Scan(&p.ID, &p.Username, &p.Rating); err != nil {
			return nil, fmt.Errorf("scan leaderboard: %w", err)
		}
		entries = append(entries, LeaderboardEntry{Player: p, Rating: p.Rating})
	}
	return entries, rows.Err()
}
