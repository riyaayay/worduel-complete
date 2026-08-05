package com.worduel.app.data

import com.squareup.moshi.Moshi
import com.squareup.moshi.kotlin.reflect.KotlinJsonAdapterFactory
import com.worduel.app.BuildConfig
import com.worduel.app.data.models.LeaderboardEntry
import com.worduel.app.data.models.QueueRequest
import com.worduel.app.data.models.QueueResponse
import com.worduel.app.data.models.QueueStatusResponse
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit

/**
 * Thin REST client over the Go backend's /api/* endpoints. Deliberately
 * hand-rolled with OkHttp + Moshi rather than Retrofit — three endpoints
 * don't need the extra abstraction, and it keeps the whole networking
 * layer readable in one place for a take-home / interview-prep project.
 */
class ApiClient(
    private val baseUrl: String = BuildConfig.API_BASE_URL
) {
    private val http = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(10, TimeUnit.SECONDS)
        .build()

    private val moshi = Moshi.Builder().add(KotlinJsonAdapterFactory()).build()
    private val jsonMediaType = "application/json".toMediaType()

    /** POST /api/queue — enter matchmaking. */
    suspend fun joinQueue(username: String): QueueResponse = withContext(Dispatchers.IO) {
        val bodyJson = moshi.adapter(QueueRequest::class.java).toJson(QueueRequest(username))
        val request = Request.Builder()
            .url("$baseUrl/api/queue")
            .post(bodyJson.toRequestBody(jsonMediaType))
            .build()
        execute(request, QueueResponse::class.java)
    }

    /** GET /api/queue/status?player_id=X — poll until matched. */
    suspend fun queueStatus(playerId: String): QueueStatusResponse = withContext(Dispatchers.IO) {
        val request = Request.Builder()
            .url("$baseUrl/api/queue/status?player_id=$playerId")
            .get()
            .build()
        execute(request, QueueStatusResponse::class.java)
    }

    /** GET /api/leaderboard */
    suspend fun leaderboard(): List<LeaderboardEntry> = withContext(Dispatchers.IO) {
        val request = Request.Builder().url("$baseUrl/api/leaderboard").get().build()
        http.newCall(request).execute().use { resp ->
            val body = resp.body?.string().orEmpty()
            val type = com.squareup.moshi.Types.newParameterizedType(
                List::class.java, LeaderboardEntry::class.java
            )
            moshi.adapter<List<LeaderboardEntry>>(type).fromJson(body) ?: emptyList()
        }
    }

    private fun <T> execute(request: Request, clazz: Class<T>): T {
        http.newCall(request).execute().use { resp ->
            val body = resp.body?.string().orEmpty()
            if (!resp.isSuccessful) {
                throw ApiException("HTTP ${resp.code}: $body")
            }
            return moshi.adapter(clazz).fromJson(body)
                ?: throw ApiException("Empty/invalid response body")
        }
    }
}

class ApiException(message: String) : Exception(message)
