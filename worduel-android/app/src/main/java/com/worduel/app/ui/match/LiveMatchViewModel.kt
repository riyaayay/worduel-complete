package com.worduel.app.ui.match

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.worduel.app.data.WordDuelEvent
import com.worduel.app.data.WordDuelWebSocket
import com.worduel.app.data.models.MatchResult
import com.worduel.app.data.models.Move
import com.worduel.app.data.models.Player
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class LiveMatchUiState(
    val board: List<String> = emptyList(),
    val players: List<Player> = emptyList(),
    val scores: Map<String, Int> = emptyMap(),
    val recentMoves: List<Move> = emptyList(),
    val secondsRemaining: Int = 90,
    val matchResult: MatchResult? = null,
    val connectionError: String? = null
)

/**
 * Owns the WebSocket connection for one match and folds incoming
 * [WordDuelEvent]s into UI state. This is the client-side mirror of the
 * backend's per-match session goroutine: exactly one place processes
 * events for this match, in order, and updates state — same "single
 * owner, no shared-mutable-state race" idea, just on the client.
 */
class LiveMatchViewModel(
    private val playerId: String,
    private val matchId: String,
    private val socket: WordDuelWebSocket = WordDuelWebSocket()
) : ViewModel() {

    private val _uiState = MutableStateFlow(LiveMatchUiState())
    val uiState: StateFlow<LiveMatchUiState> = _uiState.asStateFlow()

    val myPlayerId: String get() = playerId

    init {
        viewModelScope.launch {
            socket.connect(matchId, playerId).collect { event -> handle(event) }
        }
    }

    private fun handle(event: WordDuelEvent) {
        when (event) {
            is WordDuelEvent.MatchStart -> _uiState.value = _uiState.value.copy(
                board = event.event.board,
                players = event.event.players,
                scores = event.event.scores,
                secondsRemaining = event.event.durationSeconds
            )

            is WordDuelEvent.MoveResult -> _uiState.value = _uiState.value.copy(
                scores = event.event.scores,
                recentMoves = (listOf(event.event.move) + _uiState.value.recentMoves).take(20)
            )

            is WordDuelEvent.MatchEnd -> _uiState.value = _uiState.value.copy(
                matchResult = event.event.result
            )

            is WordDuelEvent.ConnectionError -> _uiState.value =
                _uiState.value.copy(connectionError = event.message)

            WordDuelEvent.Disconnected -> Unit
        }
    }

    /** Submit a word guess. Client-side trims/validates nothing beyond
     * non-blank — all real validation (board letters + dictionary) is the
     * server's job, same as the trie in worduel-backend. */
    fun submitWord(word: String) {
        val trimmed = word.trim()
        if (trimmed.isNotEmpty()) socket.sendMove(trimmed)
    }

    override fun onCleared() {
        super.onCleared()
        socket.disconnect()
    }
}
