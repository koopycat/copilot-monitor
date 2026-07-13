// Seed a demo database with synthetic request data for generating
// README screenshots and animated GIFs. Run before capturing the demo.
//
// Usage: go run ./demo/seed/ --db /tmp/demo.db

package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"copilot-monitoring/internal/store"
)

var models = []struct {
	name             string
	inputPrice       float64
	cachedInputPrice float64
	outputPrice      float64
	reqPct           float64
}{
	{"claude-3.5-sonnet", 3.00, 0.30, 15.00, 0.30},
	{"claude-opus-4.8", 15.00, 3.75, 75.00, 0.05},
	{"gpt-4.1", 2.00, 0.50, 8.00, 0.25},
	{"gpt-4.1-mini", 0.40, 0.10, 1.60, 0.20},
	{"o3", 0.55, 0.15, 4.40, 0.10},
	{"o4-mini", 1.10, 0.275, 4.40, 0.05},
	{"codestral", 0.20, 0.05, 1.00, 0.05},
}

func main() {
	dbPath := flag.String("db", "/tmp/demo.db", "SQLite database path")
	flag.Parse()

	_ = os.Remove(*dbPath)
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open db: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	rng := rand.New(rand.NewSource(42))
	now := time.Now()
	total := 250

	for i := 0; i < total; i++ {
		m := pickModel(rng)
		ts := now.Add(-time.Duration(rng.Intn(30*24)) * time.Hour)
		if rng.Float64() < 0.3 {
			ts = now.Add(-time.Duration(rng.Intn(7*24)) * time.Hour)
		}

		stream := 1
		if rng.Float64() < 0.1 {
			stream = 0
		}
		status := 200
		if rng.Float64() < 0.03 {
			status = 403
		}

		promptTok := 200 + rng.Intn(3800)
		cachedTok := 0
		if rng.Float64() < 0.15 {
			cachedTok = rng.Intn(promptTok)
		}
		cacheWriteTok := 0
		completionTok := 100 + rng.Intn(2400)
		totalTok := promptTok + completionTok
		latencyMs := 300 + rng.Intn(4700)
		project := ""
		providers := []string{"copilot", "copilot", "copilot", "kilo", "openai"}
		provider := providers[rng.Intn(len(providers))]

		rec := store.RequestRecord{
			Timestamp:         ts,
			Endpoint:          "chat/completions",
			Method:            "POST",
			Path:              "/" + provider + "/chat/completions",
			UpstreamHost:      "api.githubcopilot.com",
			Model:             m.name,
			Stream:            stream == 1,
			Status:            status,
			LatencyMS:         int64(latencyMs),
			PromptTokens:      promptTok,
			CachedInputTokens: cachedTok,
			CacheWriteTokens:  cacheWriteTok,
			CompletionTokens:  completionTok,
			TotalTokens:       totalTok,
			Project:           project,
			Provider:          provider,
			UsageMissing:      false,
		}
		if err := st.InsertRequest(context.Background(), rec); err != nil {
			fmt.Fprintf(os.Stderr, "insert failed: %v\n", err)
			os.Exit(1)
		}
	}

	// Recent requests (last 10 minutes) for the no-live demo so the proxy
	// log and live session show fresh activity.
	recentModels := []string{"claude-3.5-sonnet", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1", "o4-mini"}
	for i, model := range recentModels {
		ts := now.Add(-time.Duration(len(recentModels)-i) * time.Minute)
		promptTok := 500 + rng.Intn(2000)
		completionTok := 200 + rng.Intn(1000)
		rec := store.RequestRecord{
			Timestamp:        ts,
			Endpoint:         "chat/completions",
			Method:           "POST",
			Path:             "/copilot/chat/completions",
			UpstreamHost:     "api.githubcopilot.com",
			Model:            model,
			Stream:           true,
			Status:           200,
			LatencyMS:        int64(500 + rng.Intn(2000)),
			PromptTokens:     promptTok,
			CompletionTokens: completionTok,
			TotalTokens:      promptTok + completionTok,
			Provider:         "copilot",
		}
		if err := st.InsertRequest(context.Background(), rec); err != nil {
			fmt.Fprintf(os.Stderr, "insert recent failed: %v\n", err)
			os.Exit(1)
		}
	}

	if err := st.RebuildSessions(context.Background(), 30*time.Minute); err != nil {
		fmt.Fprintf(os.Stderr, "session rebuild failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Seeded %d synthetic requests into %s\n", total, *dbPath)
}

func pickModel(rng *rand.Rand) struct {
	name             string
	inputPrice       float64
	cachedInputPrice float64
	outputPrice      float64
	reqPct           float64
} {
	r := rng.Float64()
	cumulative := 0.0
	for _, m := range models {
		cumulative += m.reqPct
		if r <= cumulative {
			return m
		}
	}
	return models[0]
}
