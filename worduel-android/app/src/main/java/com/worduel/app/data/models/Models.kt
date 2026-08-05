package com.worduel.app.data.models

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

/**
 * These classes mirror the JSON shapes produced by the Go backend
 * (see worduel-backend/internal/game/types.go and internal/api/handlers.go).
 * Keeping the two in lockstep is what "native client consuming a real
 * backend" means in practice — there's no mobile-only mock API.
 */

@JsonClass(generateAdapter = true)
data class Player(
    val id: String,
    val username: String,
    val rating: Int
)

// POST /api/queue request/response
@JsonClass(generateAdapter = true)
data class QueueRequest(val username: String)

@JsonClass(generateAdapter = true)
data class QueueResponse(
    @Json(name = "player_id") val playerId: String,
    val username: String,
    val status: String
)

// GET /api/queue/status response
@JsonClass(generateAdapter = true)
data class QueueStatusResponse(
    val status: String, // "waiting" | "matched"
    @Json(name = "match_id") val matchId: String? = null
)

// GET /api/leaderboard entry
@JsonClass(generateAdapter = true)
data class LeaderboardEntry(
    val player: Player,
    val rating: Int
)

// GET /api/hint response
@JsonClass(generateAdapter = true)
data class HintResponse(
    val prefix: String,
    val words: List<String>
)

// ---- WebSocket events (server -> client) --------------------------------

/**
 * The server sends a `type` discriminator on every websocket message; we
 * decode into this loose envelope first, then re-decode the payload based
 * on `type`. See WordDuelWebSocket.kt for the dispatch.
 */
@JsonClass(generateAdapter = true)
data class MatchStartEvent(
    val type: String,
    @Json(name = "match_id") val matchId: String,
    val board: List<String>,
    val players: List<Player>,
    val scores: Map<String, Int>,
    @Json(name = "duration_seconds") val durationSeconds: Int
)

@JsonClass(generateAdapter = true)
data class Move(
    @Json(name = "player_id") val playerId: String,
    val word: String,
    val valid: Boolean,
    val score: Int,
    @Json(name = "submitted_at") val submittedAt: String
)

@JsonClass(generateAdapter = true)
data class MoveResultEvent(
    val type: String,
    val move: Move,
    val scores: Map<String, Int>
)

@JsonClass(generateAdapter = true)
data class MatchResult(
    @Json(name = "match_id") val matchId: String,
    val player1: Player,
    val player2: Player,
    val scores: Map<String, Int>,
    val moves: List<Move>,
    @Json(name = "winner_id") val winnerId: String,
    @Json(name = "started_at") val startedAt: String,
    @Json(name = "ended_at") val endedAt: String
)

@JsonClass(generateAdapter = true)
data class MatchEndEvent(
    val type: String,
    val result: MatchResult
)

// ---- WebSocket message (client -> server) --------------------------------

@JsonClass(generateAdapter = true)
data class MoveRequest(
    val type: String = "move",
    val word: String
)
