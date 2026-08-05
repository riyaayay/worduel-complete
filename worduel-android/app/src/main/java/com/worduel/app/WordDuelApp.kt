package com.worduel.app

import androidx.compose.runtime.Composable
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.worduel.app.data.models.MatchResult
import com.worduel.app.ui.match.LiveMatchScreen
import com.worduel.app.ui.matchmaking.MatchmakingScreen
import com.worduel.app.ui.results.ResultsScreen

private object Routes {
    const val MATCHMAKING = "matchmaking"
    const val LIVE_MATCH = "live_match/{playerId}/{matchId}"
    const val RESULTS = "results"

    fun liveMatch(playerId: String, matchId: String) = "live_match/$playerId/$matchId"
}

/**
 * Holds the last match result in memory so the results screen (reached via
 * navigation, not a direct object pass) can read it. A production app
 * would pass a serialized result or a shared ViewModel scoped to the nav
 * graph; this keeps the demo's navigation wiring simple.
 */
private var lastMatchResult: MatchResult? = null
private var lastMatchMyPlayerId: String = ""

@Composable
fun WordDuelApp(navController: NavHostController = rememberNavController()) {
    NavHost(navController = navController, startDestination = Routes.MATCHMAKING) {

        composable(Routes.MATCHMAKING) {
            MatchmakingScreen(
                onMatched = { playerId, matchId ->
                    navController.navigate(Routes.liveMatch(playerId, matchId))
                }
            )
        }

        composable(Routes.LIVE_MATCH) { backStackEntry ->
            val playerId = backStackEntry.arguments?.getString("playerId").orEmpty()
            val matchId = backStackEntry.arguments?.getString("matchId").orEmpty()
            LiveMatchScreen(
                playerId = playerId,
                matchId = matchId,
                onMatchEnded = { result, myPlayerId ->
                    lastMatchResult = result
                    lastMatchMyPlayerId = myPlayerId
                    navController.navigate(Routes.RESULTS) {
                        popUpTo(Routes.MATCHMAKING)
                    }
                }
            )
        }

        composable(Routes.RESULTS) {
            val result = lastMatchResult
            if (result != null) {
                ResultsScreen(
                    result = result,
                    myPlayerId = lastMatchMyPlayerId,
                    onPlayAgain = {
                        navController.navigate(Routes.MATCHMAKING) {
                            popUpTo(Routes.MATCHMAKING) { inclusive = true }
                        }
                    }
                )
            }
        }
    }
}
