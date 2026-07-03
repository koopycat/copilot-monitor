package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := "testdata.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}
	os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	schemaFiles := []string{"internal/store/schema.sql"}
	for _, sf := range schemaFiles {
		b, err := os.ReadFile(sf)
		if err != nil {
			panic(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			panic(err)
		}
	}

	rng := rand.New(rand.NewSource(42))
	now := time.Now().UTC()
	models := []struct {
		name     string
		endpoint string
		weight   int
		prompt   int
		cached   int
		output   int
		longest  int
	}{ // name, endpoint, weight, avg_prompt, avg_cached, avg_output
		{"gpt-4.1", "chat/completions", 25, 1200, 200, 800, 12},
		{"gpt-4.1-mini", "chat/completions", 15, 800, 100, 500, 8},
		{"gpt-4.1-nano", "chat/completions", 10, 400, 50, 300, 5},
		{"claude-3.5-sonnet", "chat/completions", 20, 1500, 300, 1000, 15},
		{"claude-3.5-haiku", "chat/completions", 10, 600, 80, 400, 6},
		{"o3", "chat/completions", 8, 2000, 500, 1200, 20},
		{"gpt-4.1", "responses", 5, 1000, 150, 700, 10},
		{"claude-3.5-sonnet", "responses", 3, 1200, 200, 900, 12},
		{"text-embedding-3-small", "embeddings", 4, 200, 0, 0, 2},
	}
	projects := []string{"", "my-app", "my-app", "my-app", "api-server", "api-server", "dashboard"}

	insert, err := db.Prepare(`INSERT INTO requests
(ts, endpoint, method, path, upstream_host, model, stream, status, latency_ms,
 prompt_tokens, cached_input_tokens, cache_write_tokens, completion_tokens, total_tokens, project)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		panic(err)
	}

	// Generate data over the last 400 days with varying density:
	// recent days denser, older days sparser
	total := 0
	for daysAgo := 400; daysAgo >= 0; daysAgo-- {
		requestsPerDay := 5 + rng.Intn(15)
		// More requests on recent days
		if daysAgo < 30 {
			requestsPerDay = 15 + rng.Intn(35)
		} else if daysAgo < 90 {
			requestsPerDay = 10 + rng.Intn(25)
		} else if daysAgo < 200 {
			requestsPerDay = 5 + rng.Intn(15)
		}

		dayStart := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		dayStart = time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), 0, 0, 0, 0, time.UTC)

		for i := 0; i < requestsPerDay; i++ {
			m := models[rng.Intn(len(models))]
			hour := 8 + rng.Intn(14) // 8am-10pm
			minute := rng.Intn(60)
			second := rng.Intn(60)
			ts := dayStart.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute + time.Duration(second)*time.Second)
			// Add jitter of up to 12h
			ts = ts.Add(time.Duration(rng.Intn(12)) * time.Hour)

			if ts.After(now) {
				ts = now.Add(-time.Duration(rng.Intn(3600)) * time.Second)
			}

			stream := rng.Intn(2)
			status := 200
			if rng.Intn(50) == 0 {
				status = 429
			}
			latency := 200 + rng.Intn(5000)

			prompt := jitter(rng, m.prompt, 0.4)
			cached := 0
			if rng.Intn(3) == 0 {
				cached = jitter(rng, m.cached, 0.5)
			}
			output := jitter(rng, m.output, 0.4)
			totalTok := prompt + output

			project := projects[rng.Intn(len(projects))]

			_, err := insert.Exec(
				ts.Format(time.RFC3339Nano),
				m.endpoint,
				"POST",
				"/"+m.endpoint,
				"api.githubcopilot.com",
				m.name,
				stream,
				status,
				latency,
				prompt,
				cached,
				0,
				output,
				totalTok,
				project,
			)
			if err != nil {
				panic(fmt.Errorf("insert: %w", err))
			}
			total++
		}
	}
	fmt.Printf("Seeded %d requests into %s\n", total, dbPath)
}

func jitter(rng *rand.Rand, base int, pct float64) int {
	delta := int(float64(base) * pct)
	if delta < 5 {
		delta = 5
	}
	v := base + rng.Intn(delta*2) - delta
	if v < 0 {
		v = 0
	}
	return v
}
