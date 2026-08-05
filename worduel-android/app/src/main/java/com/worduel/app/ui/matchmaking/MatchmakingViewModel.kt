package com.worduel.app.ui.matchmaking

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.worduel.app.data.ApiClient
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

sealed class MatchmakingState {
    data object Idle : MatchmakingState()
    data object Queuing : MatchmakingState()
    data class Matched(val playerId: String, val matchId: String) : MatchmakingState()
    data class Error(val message: String) : MatchmakingState()
}

/**
 * Calls POST /api/queue, then polls GET /api/queue/status once a second
 * until the matchmaking goroutine on the backend pairs this player with an
 * opponent. A production client would prefer a server push here (or a
 * long-poll), but polling keeps the client trivially simple and the
 * backend's REST surface unchanged — matchmaking latency is typically
 * sub-second anyway since pairing only waits on two queue entries.
 */
class MatchmakingViewModel(
    private val api: ApiClient = ApiClient()
) : ViewModel() {

    private val _state = MutableStateFlow<MatchmakingState>(MatchmakingState.Idle)
    val state: StateFlow<MatchmakingState> = _state.asStateFlow()

    fun findMatch(username: String) {
        if (username.isBlank()) {
            _state.value = MatchmakingState.Error("Enter a username first")
            return
        }
        _state.value = MatchmakingState.Queuing
        viewModelScope.launch {
            try {
                val queued = api.joinQueue(username)
                pollUntilMatched(queued.playerId)
            } catch (e: Exception) {
                _state.value = MatchmakingState.Error(e.message ?: "Failed to join queue")
            }
        }
    }

    private suspend fun pollUntilMatched(playerId: String) {
        repeat(60) { // ~60s timeout
            val status = api.queueStatus(playerId)
            if (status.status == "matched" && status.matchId != null) {
                _state.value = MatchmakingState.Matched(playerId, status.matchId)
                return
            }
            delay(1000)
        }
        _state.value = MatchmakingState.Error("No opponent found — try again")
    }

    fun reset() {
        _state.value = MatchmakingState.Idle
    }
}
