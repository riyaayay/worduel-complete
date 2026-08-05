package com.worduel.app.ui.results

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.worduel.app.data.models.MatchResult

@Composable
fun ResultsScreen(
    result: MatchResult,
    myPlayerId: String,
    onPlayAgain: () -> Unit
) {
    val me = if (result.player1.id == myPlayerId) result.player1 else result.player2
    val opponent = if (result.player1.id == myPlayerId) result.player2 else result.player1
    val myScore = result.scores[me.id] ?: 0
    val theirScore = result.scores[opponent.id] ?: 0

    val outcome = when {
        result.winnerId == myPlayerId -> "You won!"
        result.winnerId.isEmpty() -> "It's a tie!"
        else -> "You lost"
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Text(outcome, style = MaterialTheme.typography.headlineLarge)
        Spacer(Modifier.height(24.dp))
        Text("${me.username}: $myScore", style = MaterialTheme.typography.titleLarge)
        Text("${opponent.username}: $theirScore", style = MaterialTheme.typography.titleLarge)

        Spacer(Modifier.height(32.dp))
        Text("Words played", style = MaterialTheme.typography.titleSmall)
        result.moves.filter { it.valid }.forEach { move ->
            Text("${move.word} (+${move.score})")
        }

        Spacer(Modifier.height(32.dp))
        Button(onClick = onPlayAgain, modifier = Modifier.fillMaxWidth()) {
            Text("Play Again")
        }
    }
}
