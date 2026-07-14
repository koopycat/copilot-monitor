package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"copilot-monitoring/internal/store"
)

func TestRetentionDryRunReportsWithoutDeleting(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.InsertRequest(context.Background(), store.RequestRecord{
		Timestamp: time.Now().UTC().Add(-48 * time.Hour), Endpoint: "chat", Method: "POST", Path: "/chat",
		UpstreamHost: "api.example.test", Status: 200,
	}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	stop, dryRun, err := startRetention(st, retentionConfig{requestDays: 1, anomalyDays: 0, dryRun: true}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if !dryRun || !strings.Contains(stdout.String(), "would delete 1 requests") {
		t.Fatalf("dry run = %v, output = %q", dryRun, stdout.String())
	}
	stats, err := st.Stats(context.Background(), store.StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].Requests != 1 {
		t.Fatalf("dry run deleted data: %#v", stats)
	}
}
