# WordDuel

WordDuel is a real-time, two-player word game with a Go WebSocket backend,
PostgreSQL persistence, and a native Kotlin/Jetpack Compose Android client.

## What is included

- Matchmaking that pairs two waiting players and supports client cancellation.
- Server-authoritative scoring, dictionary validation, duplicate-word checks,
  16-tile letter-board validation, and a 90-second round timer.
- WebSocket updates for the initial board, moves, live scores, and the result.
- WebSocket match-membership checks and reconnect-safe client replacement.
- PostgreSQL persistence for players, completed matches, moves, and ratings.
- An in-memory store only for local development when `DATABASE_URL` is absent.
- A Compose Android game flow: matchmaking, live board and countdown, move feed,
  result screen, and replay.

## Run locally

### Backend only

Requires Go 1.22 or newer.

```powershell
cd worduel-backend
go run ./cmd/server
```

Without `DATABASE_URL`, the backend uses in-memory data and listens on port
`8080`. Check it with `GET /healthz`.

### Full local stack with Docker

Requires Docker Desktop.

```powershell
cd worduel-backend
docker compose up --build
```

This starts PostgreSQL, applies `schema.sql` to a fresh database volume, and
starts the API at `http://localhost:8080`. Change the sample database password
in `docker-compose.yml` before exposing it anywhere.

## Android client

Open `worduel-android` in Android Studio (JDK 17/Android SDK required). Debug
builds target `http://10.0.2.2:8080`, which is the Android emulator alias for
the host computer. For a physical device, replace the debug URLs with your
computer's LAN address and use HTTPS/WSS for any internet-facing deployment.

Release builds never permit cleartext traffic. In Android Studio, add these
Gradle properties before producing the signed release artifact:

```properties
WORDUEL_API_BASE_URL=https://api.example.com
WORDUEL_WS_BASE_URL=wss://api.example.com
```

Put them in your user Gradle properties file rather than committing them, then
use **Build → Generate Signed App Bundle / APK**.

The release defaults intentionally point at an invalid host so a build made
without configuration cannot accidentally target a development server.

## Deploy the backend

The backend expects these environment variables:

| Variable | Required in production | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | Yes | PostgreSQL URL, e.g. `postgres://user:password@host:5432/worduel?sslmode=require` |
| `PORT` | No | HTTP port, defaults to `8080` |
| `ENVIRONMENT` | Yes | Set to `production`; this makes a missing database URL a startup error |

Build and run the container from `worduel-backend`:

```powershell
docker build -t worduel-backend .
docker run --rm -p 8080:8080 `
  -e ENVIRONMENT=production `
  -e DATABASE_URL='postgres://user:password@host:5432/worduel?sslmode=require' `
  worduel-backend
```

Use a managed PostgreSQL service or run the Compose database, apply
`schema.sql` once to each new database, terminate TLS at your hosting platform
or a reverse proxy, and configure its health check as `GET /healthz`.

## Verification

From `worduel-backend`:

```powershell
go test ./...
go vet ./...
go build ./cmd/server
```

The test suite covers trie behavior plus game-session scoring, duplicate-word
rejection, completion, state snapshots, and membership checks. A live load
tool is available after starting the backend:

```powershell
go run ./cmd/loadtest -clients 20 -server http://localhost:8080
```

## Architecture

Each match has one goroutine that owns mutable match state. The matchmaking
queue and move channel serialize events; the WebSocket hub only routes them.
The PostgreSQL store persists final results after the round, keeping live
gameplay independent from database latency.
