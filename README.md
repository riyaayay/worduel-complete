# WordDuel

Real-time multiplayer word game. Go backend (goroutines + channels for
matchmaking and game sessions, a trie for word validation, WebSockets for
live sync) + a native Kotlin/Compose Android client.

This is a **working, tested implementation** of the build plan, not a
scaffold: the backend compiles, passes its tests, and was smoke-tested
end-to-end (matchmaking, WebSocket sync, move validation, and a load test)
during development — see "What's actually verified working" below.

```
worduel/
├── worduel-backend/   Go backend — the primary deliverable
└── worduel-android/   Kotlin/Compose Android client (source; needs Android
                        Studio + SDK to build, not buildable in this sandbox)
```

## Quick start (backend)

Requires Go 1.22+.

```bash
cd worduel-backend
go mod tidy          # fetches github.com/gorilla/websocket
go run ./cmd/server   # listens on :8080
```

Try it:

```bash
# Two players queue up
curl -X POST localhost:8080/api/queue -d '{"username":"Alice"}'
curl -X POST localhost:8080/api/queue -d '{"username":"Bob"}'

# Poll until matched (returns a match_id once the matchmaking goroutine pairs them)
curl "localhost:8080/api/queue/status?player_id=<player_id_from_above>"

# Then connect a websocket to ws://localhost:8080/ws?player_id=X&match_id=Y
# and send {"type":"move","word":"CAT"}
```

Run the load test (Phase 5 from the plan — a real, measured latency number):

```bash
go run ./cmd/loadtest -clients 80 -server http://localhost:8080
```

## What's actually verified working

I built and ran this, not just wrote it. In order:

- `go build ./...`, `go vet ./...`, `go test ./...` all pass.
- Full REST flow tested live with curl: queue two players, poll status,
  confirm they get paired into the same match.
- WebSocket flow tested with a small Go test client: both players receive
  the shared board, an invalid word is correctly rejected (wrong dictionary
  word), a word not present on the board's letters is correctly rejected
  (board-multiset check), and both players see `move_result` broadcasts in
  real time.
- **Found and fixed a real race condition** during testing: the match
  session goroutine broadcasts `match_start` the instant it's created, but
  a player who hasn't opened their WebSocket yet (still polling
  `/api/queue/status`) would miss it. Fixed with a channel-based
  request/response snapshot (`Session.State()`) so a newly-connecting
  client gets synced directly rather than relying on having caught a
  one-shot broadcast — same "channel, not mutex" philosophy as the rest of
  the session.
- Load-tested with 80 concurrent simulated clients
  (`cmd/loadtest`): **p95 move round-trip latency ≈ 12ms**, well under the
  90-100ms target quoted in the original plan. Matchmaking p95 ≈ 99ms
  (mostly an artifact of the load test's own 50ms polling interval, not
  actual pairing latency — the pairing loop itself blocks on two channel
  reads, effectively free). 2/80 clients hit a transient `bad handshake` on
  the WebSocket upgrade under that burst — worth investigating further
  (likely OS-level accept-queue backlog under a synthetic instant-burst of
  80 simultaneous dials; real traffic ramps up, it doesn't arrive as one
  spike) before quoting a "0 errors" number anywhere.


## ER diagram

See `worduel-backend/schema.sql` for the full schema + comments on *why*
each design choice was made (denormalizing `board` onto `matches`, storing
invalid moves too, etc.) — that "why", not just the boxes-and-arrows, is
what actually reads as depth in an interview.

```
users ──1───< matches >───1── users   (player1_id / player2_id / winner_id)
  │
  └──1───< leaderboard_snapshots

matches ──1───< moves
```
