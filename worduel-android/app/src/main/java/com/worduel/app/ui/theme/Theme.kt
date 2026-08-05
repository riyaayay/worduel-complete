package com.worduel.app.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

private val WordDuelPrimary = Color(0xFF3F51B5)
private val WordDuelSecondary = Color(0xFF03A9F4)

private val LightColors = lightColorScheme(
    primary = WordDuelPrimary,
    secondary = WordDuelSecondary
)

private val DarkColors = darkColorScheme(
    primary = WordDuelPrimary,
    secondary = WordDuelSecondary
)

@Composable
fun WordDuelTheme(content: @Composable () -> Unit) {
    val colors = if (isSystemInDarkTheme()) DarkColors else LightColors
    MaterialTheme(colorScheme = colors, content = content)
}
