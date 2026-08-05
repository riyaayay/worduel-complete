package com.worduel.app.data

import com.squareup.moshi.Moshi
import com.squareup.moshi.Types
import com.squareup.moshi.kotlin.reflect.KotlinJsonAdapterFactory
import com.worduel.app.BuildConfig
import com.worduel.app.data.models.MatchEndEvent
import com.worduel.app.data.models.MatchStartEvent
import com.worduel.app.data.models.MoveRequest
import com.worduel.app.data.models.MoveResultEvent
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okio.ByteString

/** The three event types the server can push over the socket. */
sealed class WordDuelEvent {
    data class MatchStart(val event: MatchStartEvent) : WordDuelEvent()
    data class MoveResult(val event: MoveResultEvent) : WordDuelEvent()
    data class MatchEnd(val event: MatchEndEvent) : WordDuelEvent()
    data class ConnectionError(val message: String) : WordDuelEvent()
    data object Disconnected : WordDuelEvent()
}

/**
 * Wraps an OkHttp WebSocket connection to a single match as a cold Flow of
 * [WordDuelEvent]s, and exposes [sendMove] for submitting words. This is
 * the real-time counterpart to [ApiClient] — matchmaking and leaderboard
 * are request/response, but gameplay itself is push-based.
 */
class WordDuelWebSocket(
    private val wsBaseUrl: String = BuildConfig.WS_BASE_URL
) {
    private val client = OkHttpClient()
    private val moshi = Moshi.Builder().add(KotlinJsonAdapterFactory()).build()
    private var socket: WebSocket? = null

    /**
     * Connects to a specific match and emits every event received until the
     * flow is cancelled (e.g. the composable leaves the live-match screen)
     * or the server closes the connection (match ended).
     */
    fun connect(matchId: String, playerId: String): Flow<WordDuelEvent> = callbackFlow {
        val url = "$wsBaseUrl/ws?match_id=$matchId&player_id=$playerId"
        val request = Request.Builder().url(url).build()

        val listener = object : WebSocketListener() {
            override fun onMessage(webSocket: WebSocket, text: String) {
                val event = decode(text)
                if (event != null) trySend(event)
            }

            override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
                onMessage(webSocket, bytes.utf8())
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: okhttp3.Response?) {
                trySend(WordDuelEvent.ConnectionError(t.message ?: "unknown websocket error"))
                close(t)
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                trySend(WordDuelEvent.Disconnected)
                close()
            }
        }

        socket = client.newWebSocket(request, listener)

        awaitClose {
            socket?.close(1000, "client leaving match screen")
            socket = null
        }
    }

    /** Submit a word guess for validation over the open connection. */
    fun sendMove(word: String) {
        val payload = moshi.adapter(MoveRequest::class.java).toJson(MoveRequest(word = word))
        socket?.send(payload)
    }

    fun disconnect() {
        socket?.close(1000, "client disconnect")
        socket = null
    }

    /**
     * Server messages carry a `type` discriminator. Peek at it with a loose
     * Map decode, then re-decode into the concrete class — simpler than a
     * custom Moshi polymorphic adapter for three event types.
     */
    private fun decode(text: String): WordDuelEvent? {
        val mapType = Types.newParameterizedType(Map::class.java, String::class.java, Any::class.java)
        val raw = moshi.adapter<Map<String, Any>>(mapType).fromJson(text) ?: return null
        return when (raw["type"]) {
            "match_start" -> moshi.adapter(MatchStartEvent::class.java).fromJson(text)
                ?.let { WordDuelEvent.MatchStart(it) }
            "move_result" -> moshi.adapter(MoveResultEvent::class.java).fromJson(text)
                ?.let { WordDuelEvent.MoveResult(it) }
            "match_end" -> moshi.adapter(MatchEndEvent::class.java).fromJson(text)
                ?.let { WordDuelEvent.MatchEnd(it) }
            else -> null
        }
    }
}
