// Command loadtest is Phase 5 from the build plan: spin up N concurrent
// fake clients against a running WordDuel server, queue them all up,
// connect their websockets once matched, submit a move each, and report
// p50/p95/max latency for both matchmaking and move round-trip.
//
// Usage:
//
//	go run ./cmd/loadtest -clients 100 -server http://localhost:8080
//
// This gives a real, defensible latency number instead of a guessed one
// for your resume bullet.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type queueResp struct {
	PlayerID string `json:"player_id"`
}

type statusResp struct {
	Status  string `json:"status"`
	MatchID string `json:"match_id"`
}

func main() {
	clients := flag.Int("clients", 50, "number of concurrent simulated clients (must be even)")
	server := flag.String("server", "http://localhost:8080", "WordDuel server base URL")
	flag.Parse()

	if *clients%2 != 0 {
		*clients++
	}
	wsServer := "ws" + strings.TrimPrefix(*server, "http")

	var (
		mu               sync.Mutex
		matchmakingTimes []time.Duration
		moveTimes        []time.Duration
		errs             int
	)

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			username := fmt.Sprintf("loadtest_%d", n)

			t0 := time.Now()
			playerID, err := joinQueue(*server, username)
			if err != nil {
				mu.Lock()
				errs++
				mu.Unlock()
				log.Printf("client %d: queue error: %v", n, err)
				return
			}

			matchID, err := pollUntilMatched(*server, playerID)
			if err != nil {
				mu.Lock()
				errs++
				mu.Unlock()
				log.Printf("client %d: matchmaking error: %v", n, err)
				return
			}
			matchmakingLatency := time.Since(t0)

			moveLatency, err := connectAndMove(wsServer, playerID, matchID)
			if err != nil {
				mu.Lock()
				errs++
				mu.Unlock()
				log.Printf("client %d: websocket error: %v", n, err)
				return
			}

			mu.Lock()
			matchmakingTimes = append(matchmakingTimes, matchmakingLatency)
			moveTimes = append(moveTimes, moveLatency)
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	total := time.Since(start)

	fmt.Printf("\n=== WordDuel load test: %d clients ===\n", *clients)
	fmt.Printf("total wall time: %v\n", total)
	fmt.Printf("errors: %d\n", errs)
	report("matchmaking latency (queue -> matched)", matchmakingTimes)
	report("move round-trip latency (submit -> broadcast received)", moveTimes)
}

func joinQueue(server, username string) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": username})
	resp, err := http.Post(server+"/api/queue", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var qr queueResp
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return "", err
	}
	return qr.PlayerID, nil
}

func pollUntilMatched(server, playerID string) (string, error) {
	for i := 0; i < 100; i++ {
		resp, err := http.Get(server + "/api/queue/status?player_id=" + playerID)
		if err != nil {
			return "", err
		}
		var sr statusResp
		json.NewDecoder(resp.Body).Decode(&sr)
		resp.Body.Close()
		if sr.Status == "matched" {
			return sr.MatchID, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for match")
}

func connectAndMove(wsServer, playerID, matchID string) (time.Duration, error) {
	u := url.URL{Scheme: "ws", Host: strings.TrimPrefix(wsServer, "ws://"), Path: "/ws",
		RawQuery: "player_id=" + playerID + "&match_id=" + matchID}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// Drain the initial match_start sync message.
	conn.ReadMessage()

	t0 := time.Now()
	payload, _ := json.Marshal(map[string]string{"type": "move", "word": "TEST"})
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return 0, err
	}
	// Wait for the move_result broadcast (own move reflected back).
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		return 0, err
	}
	return time.Since(t0), nil
}

func report(label string, times []time.Duration) {
	if len(times) == 0 {
		fmt.Printf("%s: no samples\n", label)
		return
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	p50 := times[len(times)*50/100]
	p95 := times[min(len(times)*95/100, len(times)-1)]
	max := times[len(times)-1]
	fmt.Printf("%s:\n  p50=%v  p95=%v  max=%v  (n=%d)\n", label, p50, p95, max, len(times))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
