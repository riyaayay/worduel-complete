package com.worduel.app.ui.matchmaking

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel

/**
 * Screen 1 of 3: enter a username, tap "Find Match", wait for the backend's
 * matchmaking goroutine to pair you with an opponent.
 */
@Composable
fun MatchmakingScreen(
    onMatched: (playerId: String, matchId: String) -> Unit,
    viewModel: MatchmakingViewModel = viewModel()
) {
    val state by viewModel.state.collectAsState()
    var username by remember { mutableStateOf("") }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Text("WordDuel", style = MaterialTheme.typography.headlineLarge)
        Spacer(Modifier.height(32.dp))

        when (val s = state) {
            is MatchmakingState.Idle, is MatchmakingState.Error -> {
                OutlinedTextField(
                    value = username,
                    onValueChange = { username = it },
                    label = { Text("Username") },
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(Modifier.height(16.dp))
                Button(
                    onClick = { viewModel.findMatch(username) },
                    modifier = Modifier.fillMaxWidth()
                ) { Text("Find Match") }

                if (s is MatchmakingState.Error) {
                    Spacer(Modifier.height(12.dp))
                    Text(s.message, color = MaterialTheme.colorScheme.error)
                }
            }

            is MatchmakingState.Queuing -> {
                CircularProgressIndicator()
                Spacer(Modifier.height(16.dp))
                Text("Finding an opponent...")
            }

            is MatchmakingState.Matched -> {
                // Fire the navigation callback once, then let the parent
                // move us to the live match screen.
                onMatched(s.playerId, s.matchId)
            }
        }
    }
}
