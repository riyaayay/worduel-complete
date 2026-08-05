-- WordDuel — PostgreSQL schema
--
-- This is the schema referenced throughout the build plan as "the one
-- you'll draw live in the Round 2 interview". It's deliberately small
-- (4 tables) so you can sketch it from memory on a whiteboard, but it's a
-- real normalized design: 1 users, many matches per user, many moves per
-- match, with foreign keys and indexes chosen for the actual query
-- patterns the app needs (match replay, leaderboard, rating history).
--
-- ER diagram (draw this):
--
--   users ──1───< matches >───1── users        (player1_id / player2_id / winner_id, all -> users.id)
--     │
--     └──1───< leaderboard_snapshots
--
--   matches ──1───< moves

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(32) NOT NULL UNIQUE,
    email         VARCHAR(255) UNIQUE,
    rating        INTEGER NOT NULL DEFAULT 1000,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE matches (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player1_id    UUID NOT NULL REFERENCES users(id),
    player2_id    UUID NOT NULL REFERENCES users(id),
    winner_id     UUID REFERENCES users(id),        -- NULL = tie
    board         VARCHAR(16) NOT NULL,              -- the 16 letter tiles used, e.g. "OMBYPROABYIGIYUY"
    started_at    TIMESTAMPTZ NOT NULL,
    ended_at      TIMESTAMPTZ,
    CONSTRAINT different_players CHECK (player1_id <> player2_id)
);

CREATE TABLE moves (
    id            BIGSERIAL PRIMARY KEY,
    match_id      UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    player_id     UUID NOT NULL REFERENCES users(id),
    word          VARCHAR(32) NOT NULL,
    valid         BOOLEAN NOT NULL,
    score         INTEGER NOT NULL DEFAULT 0,
    submitted_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Optional: periodic rating snapshots, useful for a rating-over-time chart
-- on a player's profile screen.
CREATE TABLE leaderboard_snapshots (
    id            BIGSERIAL PRIMARY KEY,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating        INTEGER NOT NULL,
    snapshot_date DATE NOT NULL DEFAULT CURRENT_DATE
);

-- Fast match-replay queries: "give me all moves in match X in order".
CREATE INDEX idx_moves_match_submitted ON moves (match_id, submitted_at);

-- Fast "match history for user" queries in either player slot.
CREATE INDEX idx_matches_player1 ON matches (player1_id);
CREATE INDEX idx_matches_player2 ON matches (player2_id);

-- Fast leaderboard reads straight from Postgres if Redis is ever cold.
CREATE INDEX idx_users_rating ON users (rating DESC);

-- ---------------------------------------------------------------------
-- Notes for the interview:
-- * `board` is denormalized onto `matches` (rather than a separate
--   `board_tiles` table) because it's write-once, read-with-the-match,
--   and never queried independently — a textbook case for denormalizing.
-- * `moves.valid` is stored even for rejected words. This is intentional:
--   it lets you replay exactly what the player attempted, and lets you
--   compute stats like "attempt accuracy" later without re-deriving them.
-- * In production, active match state lives in Redis
--   (HSET match:{id} board ... scores ...) and only gets written here once
--   on match end — Postgres is the durable system of record, Redis is the
--   hot path. See Phase 3 in the build plan for the "why Redis" answer
--   (O(log n) ZADD/ZREVRANGE for the leaderboard vs. an ORDER BY scan).
-- ---------------------------------------------------------------------
