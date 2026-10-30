package com.worduel.app.ui.match

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.worduel.app.data.models.MatchResult
import com.worduel.app.data.models.Move
import com.worduel.app.data.models.Player
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.time.Instant

private data class PracticeRound(val tiles: String, val words: Set<String>)

private val practiceRounds = listOf(
    PracticeRound("CATDOGSRELINMPUB", setOf("CAT", "DOG", "CATS", "CARD", "CARE", "SCARE", "STAR", "RAIN", "RING", "LINE", "LION", "MIND", "MINE", "PINE", "PINS", "BUMP", "BUNS")),
    PracticeRound("PLANETSWORDQUICK", setOf("PLAN", "PLANE", "PLANET", "STONE", "TONE", "NOTE", "WORD", "WORDS", "SWORD", "QUICK", "QUIT", "QUIET", "WIND", "WINE", "LINE", "LION")),
    PracticeRound("GAMESFLOWERBRICK", setOf("GAME", "GAMES", "FLOW", "FLOWER", "LOWER", "GLOW", "GLOBE", "BRICK", "BICKER", "RICE", "FIRE", "FIRE", "CROW", "CROW", "CROWE", "WORK", "WORKER"))
)

private val localLetterScores = mapOf(
    'A' to 1, 'E' to 1, 'I' to 1, 'O' to 1, 'U' to 1, 'L' to 1, 'N' to 1, 'S' to 1, 'T' to 1, 'R' to 1,
    'D' to 2, 'G' to 2, 'B' to 3, 'C' to 3, 'M' to 3, 'P' to 3, 'F' to 4, 'H' to 4, 'V' to 4, 'W' to 4,
    'Y' to 4, 'K' to 5, 'J' to 8, 'X' to 8, 'Q' to 10, 'Z' to 10
)

/** A self-contained practice round for APK demos without a running backend. */
class LocalMatchViewModel(username: String) : ViewModel() {
    private val round = practiceRounds.random()
    private val startedAt = Instant.now().toString()
    private val me = Player("local-player", username, 1000)
    private val bot = Player("word-bot", "WordBot", 1000)
    private val scores = mutableMapOf(me.id to 0, bot.id to 0)
    private val found = mutableSetOf<String>()
    private val moves = mutableListOf<Move>()
    private var timerJob: Job? = null
    private var botJob: Job? = null

    private val _uiState = MutableStateFlow(
        LiveMatchUiState(
            board = round.tiles.map { it.toString() },
            players = listOf(me, bot),
            scores = scores.toMap(),
            secondsRemaining = ROUND_SECONDS
        )
    )
    val uiState: StateFlow<LiveMatchUiState> = _uiState.asStateFlow()
    val myPlayerId: String get() = me.id
    val practiceWords: String get() = round.words.sorted().joinToString(", ")

    init {
        startClock()
        startBot()
    }

    fun submitWord(input: String) {
        val word = input.trim().uppercase()
        if (word.isNotEmpty()) applyMove(me.id, word)
    }

    private fun startClock() {
        timerJob = viewModelScope.launch {
            repeat(ROUND_SECONDS) {
                delay(1_000)
                _uiState.value = _uiState.value.copy(
                    secondsRemaining = (_uiState.value.secondsRemaining - 1).coerceAtLeast(0)
                )
            }
            finishMatch()
        }
    }

    private fun startBot() {
        botJob = viewModelScope.launch {
            round.words.shuffled().take(8).forEach { word ->
                delay(4_000)
                if (_uiState.value.matchResult == null) applyMove(bot.id, word)
            }
        }
    }

    private fun applyMove(playerId: String, word: String) {
        if (_uiState.value.matchResult != null) return
        val key = "$playerId:$word"
        val valid = word.length >= 3 && word in round.words && canFormWord(word) && found.add(key)
        val points = if (valid) word.sumOf { localLetterScores[it] ?: 0 } else 0
        if (valid) scores[playerId] = (scores[playerId] ?: 0) + points
        val move = Move(playerId, word, valid, points, Instant.now().toString())
        moves += move
        _uiState.value = _uiState.value.copy(
            scores = scores.toMap(),
            recentMoves = (listOf(move) + _uiState.value.recentMoves).take(20)
        )
    }

    private fun canFormWord(word: String): Boolean {
        val counts = round.tiles.groupingBy { it }.eachCount().toMutableMap()
        return word.all { letter -> (counts[letter] ?: 0).let { count ->
            if (count == 0) false else {
                counts[letter] = count - 1
                true
            }
        } }
    }

    private fun finishMatch() {
        if (_uiState.value.matchResult != null) return
        botJob?.cancel()
        val winner = when {
            (scores[me.id] ?: 0) > (scores[bot.id] ?: 0) -> me.id
            (scores[bot.id] ?: 0) > (scores[me.id] ?: 0) -> bot.id
            else -> ""
        }
        _uiState.value = _uiState.value.copy(
            matchResult = MatchResult(
                matchId = "local-practice",
                player1 = me,
                player2 = bot,
                scores = scores.toMap(),
                moves = moves.toList(),
                winnerId = winner,
                startedAt = startedAt,
                endedAt = Instant.now().toString()
            )
        )
    }

    override fun onCleared() {
        timerJob?.cancel()
        botJob?.cancel()
    }

    private companion object {
        const val ROUND_SECONDS = 90
    }
}
