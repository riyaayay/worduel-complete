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

## What's stubbed vs. real 

| Piece | Status |
|---|---|
| Trie word validation | **Real.** 63,612-word dictionary (derived from the system's American English word list) embedded in the binary. O(k) lookups, tested. |
| Matchmaking (channels + goroutines) | **Real.** Buffered channel queue, single pairing goroutine, verified live. |
| Game session (goroutine-owned state) | **Real.** No mutex on match state — verified via the race-condition fix above, which is itself good interview material ("tell me about a concurrency bug you found"). |
| WebSocket real-time sync | **Real.** Verified with a live two-client test. |
| PostgreSQL | **Schema only** (`worduel-backend/schema.sql`). The running server uses an in-memory store behind a `Store` interface — swapping in a real Postgres-backed implementation is additive, not a rewrite. Say this plainly if asked. |
| Redis | **Not implemented**, described only in the plan/README. If you want it, it's a genuinely small addition: cache `Session.snapshot()` output in a `match:{id}` hash on each move, and use a sorted set for the leaderboard instead of the in-memory store's `sort.Slice`. |
| Android client | **Full MVVM source**, written and internally consistent with the backend's actual JSON shapes (I cross-checked field names against the Go structs). **Not compiled** — this sandbox has no Android SDK. Open it in Android Studio, point `BuildConfig.API_BASE_URL` / `WS_BASE_URL` at your running backend (defaults to the emulator's `10.0.2.2` alias), and build. If you can't get to this before your interview, the honest move — per the original plan — is to scope your resume bullet to the backend only. |
| Load test | **Real**, ran it, numbers above are from an actual run, not guessed. |

## Architecture notes for interview talking points

- **Why channels instead of mutexes for matchmaking/sessions**: a channel
  is both the data structure and the synchronization primitive. The
  matchmaking queue is a buffered channel; pairing is "read twice, pair
  them" with no lock. Each match session is a goroutine that owns its state
  exclusively — moves arrive on a channel and are processed one at a time,
  so there's no shared mutable state to race on. The one place I *do* use a
  mutex (`ws.Hub`'s connection registry, `store.MemoryStore`) is
  deliberate: those really are shared across many goroutines reading and
  writing concurrently, and a mutex is the right tool there — not
  everything should be channels.
- **Why a trie, not a hash set, for the dictionary**: `IsValidWord` is O(k)
  either way for the k-length word being checked, but the trie also gives
  `WordsWithPrefix` almost for free (a DFS from the prefix node) — that's
  the hint feature, and a second, richer trie use case to talk about beyond
  "I did a lookup."
- **Why the board is a 16-letter multiset check, not a full Boggle
  adjacency graph**: intentional scope cut to keep move validation O(word
  length) instead of adding a path-search over the board. Worth mentioning
  as a deliberate simplification if asked, not something you're unaware of.

## ER diagram (draw this live)

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
