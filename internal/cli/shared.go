package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

var ymdRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// projectDefault returns the default project name from the environment.
func projectDefault() string {
	return os.Getenv("COPILOT_MONITOR_PROJECT")
}

func parseSince(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if value == "" || lower == "all" {
		return time.Time{}, nil
	}
	// Absolute date: YYYY-MM-DD
	if ymdRe.MatchString(value) {
		t, err := time.Parse("2006-01-02", value)
		if err != nil {
			return time.Time{}, err
		}
		// Treat this as the start of the day in the local timezone.
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location()), nil
	}
	if strings.HasSuffix(lower, "d") {
		daysText := strings.TrimSuffix(lower, "d")
		var days int
		if _, err := fmt.Sscanf(daysText, "%d", &days); err != nil {
			return time.Time{}, err
		}
		if days < 0 {
			return time.Time{}, fmt.Errorf("duration must be non-negative")
		}
		return now.Add(-time.Duration(days) * 24 * time.Hour), nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return time.Time{}, err
	}
	if d < 0 {
		return time.Time{}, fmt.Errorf("duration must be non-negative")
	}
	return now.Add(-d), nil
}
