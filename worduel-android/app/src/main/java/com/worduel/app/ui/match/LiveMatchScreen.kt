package com.worduel.app.ui.match

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewmodel.compose.viewModel
import com.worduel.app.data.models.MatchResult

class LiveMatchViewModelFactory(
    private val playerId: String,
    private val matchId: String
) : ViewModelProvider.Factory {
    @Suppress("UNCHECKED_CAST")
    override fun <T : ViewModel> create(modelClass: Class<T>): T =
        LiveMatchViewModel(playerId, matchId) as T
}

@Composable
fun LiveMatchScreen(
    playerId: String,
    matchId: String,
    onMatchEnded: (MatchResult, myPlayerId: String) -> Unit
) {
    val viewModel: LiveMatchViewModel = viewModel(
        factory = LiveMatchViewModelFactory(playerId, matchId)
    )
    val state by viewModel.uiState.collectAsState()
    var wordInput by remember { mutableStateOf("") }

    LaunchedEffect(state.matchResult) {
        state.matchResult?.let { onMatchEnded(it, viewModel.myPlayerId) }
    }

    Column(modifier = Modifier.fillMaxSize().padding(16.dp)) {
        // Scoreboard
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            state.players.forEach { p ->
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(
                        p.username,
                        style = if (p.id == playerId) MaterialTheme.typography.titleMedium
                                else MaterialTheme.typography.bodyMedium
                    )
                    Text(
                        "${state.scores[p.id] ?: 0}",
                        style = MaterialTheme.typography.headlineMedium
                    )
                }
            }
        }

        Spacer(Modifier.height(24.dp))

        // Shared letter board
        LazyVerticalGrid(
            columns = GridCells.Fixed(4),
            modifier = Modifier.fillMaxWidth()
        ) {
            items(state.board) { letter ->
                Card(modifier = Modifier.padding(4.dp)) {
                    Text(
                        letter,
                        modifier = Modifier.padding(20.dp),
                        style = MaterialTheme.typography.headlineSmall
                    )
                }
            }
        }

        Spacer(Modifier.height(16.dp))

        // Word submission
        Row(verticalAlignment = Alignment.CenterVertically) {
            OutlinedTextField(
                value = wordInput,
                onValueChange = { wordInput = it.uppercase() },
                label = { Text("Enter a word") },
                modifier = Modifier.weight(1f)
            )
            Spacer(Modifier.height(8.dp))
            Button(onClick = {
                viewModel.submitWord(wordInput)
                wordInput = ""
            }) { Text("Submit") }
        }

        Spacer(Modifier.height(16.dp))
        state.connectionError?.let {
            Text("Connection issue: $it", color = MaterialTheme.colorScheme.error)
        }

        // Live move feed — this is where you actually see the opponent's
        // moves land in real time via the websocket broadcast.
        Text("Recent moves", style = MaterialTheme.typography.titleSmall)
        LazyColumn(modifier = Modifier.fillMaxWidth()) {
            items(state.recentMoves) { move ->
                val who = state.players.firstOrNull { it.id == move.playerId }?.username ?: move.playerId
                val outcome = if (move.valid) "+${move.score}" else "invalid"
                Text("$who: ${move.word} ($outcome)")
            }
        }
    }
}
