package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"copilot-monitoring/internal/store"
)

type retentionConfig struct {
	requestDays int
	anomalyDays int
	dryRun      bool
}

// startRetention performs the startup retention check and returns a function
// which stops the daily check. A zero retention value disables that data type.
func startRetention(st *store.Store, cfg retentionConfig, stdout, stderr io.Writer) (func(), bool, error) {
	st.SetRetentionDays(cfg.requestDays)
	if cfg.requestDays < 0 || cfg.anomalyDays < 0 {
		return nil, false, fmt.Errorf("retention days must be zero or greater")
	}

	run := func() error {
		now := time.Now().UTC()
		var requestBefore, anomalyBefore time.Time
		if cfg.requestDays > 0 {
			requestBefore = now.AddDate(0, 0, -cfg.requestDays)
		}
		if cfg.anomalyDays > 0 {
			anomalyBefore = now.AddDate(0, 0, -cfg.anomalyDays)
		}
		counts, err := st.PrunableCounts(context.Background(), requestBefore, anomalyBefore)
		if err != nil {
			return err
		}
		total, err := st.RetentionRowCount(context.Background())
		if err != nil {
			return err
		}
		if total > 0 && counts.Total()*2 > total {
			fmt.Fprintf(stderr, "warning: retention would delete %d of %d database rows (>50%%)\n", counts.Total(), total)
		}
		if cfg.dryRun {
			fmt.Fprintf(stdout, "retention dry run: would delete %d requests, %d sessions, %d anomalies\n", counts.Requests, counts.Sessions, counts.Anomalies)
			return nil
		}

		requests, err := st.PruneRequests(context.Background(), requestBefore)
		if err != nil {
			return err
		}
		anomalies, err := st.PruneAnomalies(context.Background(), anomalyBefore)
		if err != nil {
			return err
		}
		st.RecordPrune(now, requests+anomalies)
		return nil
	}

	if err := run(); err != nil {
		return nil, false, err
	}
	if cfg.dryRun {
		return func() {}, true, nil
	}

	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if err := run(); err != nil {
					fmt.Fprintf(stderr, "error: retention prune: %v\n", err)
				}
			}
		}
	}()
	return func() {
		select {
		case <-stop:
		default:
			close(stop)
		}
	}, false, nil
}
