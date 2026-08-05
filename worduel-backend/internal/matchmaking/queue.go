// Package matchmaking pairs waiting players using a buffered channel as the
// queue and a single goroutine as the pairing loop. This is the core "Go
// concurrency" story of WordDuel: the channel *is* the synchronization
// primitive, so there's no lock guarding a queue slice.
package matchmaking

import (
	"log"

	"worduel-backend/internal/game"
)

// Ticket is what a player submits when they want a match.
type Ticket struct {
	Player game.Player
}

// SessionFactory creates and starts (as its own goroutine) a new match
// between two players, returning the session so the caller can register it
// (e.g. so the websocket hub knows which session owns which match id).
type SessionFactory func(p1, p2 game.Player) *game.Session

// Queue is the matchmaking service.
type Queue struct {
	tickets chan Ticket
	newMatch chan *game.Session
	factory  SessionFactory
}

// New creates a matchmaking queue. bufferSize controls how many players can
// be waiting in the channel simultaneously before Enqueue blocks — a simple,
// explicit backpressure mechanism.
func New(bufferSize int, factory SessionFactory) *Queue {
	return &Queue{
		tickets:  make(chan Ticket, bufferSize),
		newMatch: make(chan *game.Session, bufferSize),
		factory:  factory,
	}
}

// Enqueue submits a player into the matchmaking pool. Non-blocking up to
// the channel's buffer size.
func (q *Queue) Enqueue(t Ticket) {
	q.tickets <- t
}

// Matches is a read-only stream of newly created sessions, consumed by
// whatever component tracks "which match id is player X in" (the API
// layer, in this project).
func (q *Queue) Matches() <-chan *game.Session {
	return q.newMatch
}

// Run is the single pairing-loop goroutine: it blocks reading two tickets
// off the channel, pairs them, and spawns a session goroutine to own that
// match's game state. Call this once, e.g. `go queue.Run()`.
func (q *Queue) Run() {
	for {
		first := <-q.tickets
		second := <-q.tickets

		// Never pair a player against themselves if they somehow got
		// enqueued twice in a row.
		if first.Player.ID == second.Player.ID {
			q.tickets <- second
			continue
		}

		session := q.factory(first.Player, second.Player)
		log.Printf("matchmaking: paired %s vs %s -> match %s",
			first.Player.Username, second.Player.Username, session.ID)

		go session.Run()
		q.newMatch <- session
	}
}
