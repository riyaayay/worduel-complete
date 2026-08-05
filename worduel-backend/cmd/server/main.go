// Command server is WordDuel's entrypoint: it loads the dictionary trie,
// wires up the matchmaking queue / websocket hub / store, and serves the
// REST + WebSocket API.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"worduel-backend/internal/api"
	"worduel-backend/internal/matchmaking"
	"worduel-backend/internal/store"
	"worduel-backend/internal/trie"
	"worduel-backend/internal/ws"
)

func main() {
	log.Println("loading dictionary...")
	dict := trie.LoadDefaultDictionary()
	log.Printf("dictionary loaded: %d words", dict.Size())

	hub := ws.NewHub()
	memStore := store.NewMemoryStore()

	// The queue needs a session factory, and the factory closes over the
	// App (hub/dict/store) — so build the App first, then the queue, then
	// attach the queue back to the App.
	app := api.NewApp(hub, dict, memStore)
	queue := matchmaking.New(64, app.NewSessionFactory())
	app.SetQueue(queue)
	go queue.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/queue", app.HandleQueue)
	mux.HandleFunc("/api/queue/status", app.HandleQueueStatus)
	mux.HandleFunc("/api/leaderboard", app.HandleLeaderboard)
	mux.HandleFunc("/api/hint", app.HandleHint)
	mux.HandleFunc("/ws", app.HandleWS)
	mux.HandleFunc("/healthz", app.HandleHealth)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      withCORS(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	log.Printf("WordDuel backend listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

// withCORS is a permissive CORS wrapper suitable for local development
// against the Android client / a browser test client. Tighten for
// production.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
