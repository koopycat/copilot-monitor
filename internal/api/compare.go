package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"copilot-monitoring/internal/catalog"
	costcalc "copilot-monitoring/internal/cost"
	"copilot-monitoring/internal/store"
)

type compareResponse struct {
	Periods []comparePeriodResponse `json:"periods"`
}

type comparePeriodResponse struct {
	Label       string             `json:"label"`
	Start       time.Time          `json:"start"`
	End         time.Time          `json:"end"`
	Models      []store.ModelStats `json:"models"`
	Requests    int                `json:"requests"`
	TotalTokens int                `json:"total_tokens"`
	TotalCost   float64            `json:"total_cost"`
}

func (h *Handler) handleCompare(w http.ResponseWriter, r *http.Request) {
	aStart, aEnd, bStart, bEnd, err := parseCompareWindows(r, time.Now().UTC())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.db.CompareStats(r.Context(), aStart, aEnd, bStart, bEnd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cat, err := catalog.LoadDefault()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := compareResponse{Periods: make([]comparePeriodResponse, 0, len(result.Periods))}
	for _, period := range result.Periods {
		cost := costcalc.Calculate(period.Models, cat)
		response.Periods = append(response.Periods, comparePeriodResponse{
			Label:       period.Label,
			Start:       period.Start,
			End:         period.End,
			Models:      period.Models,
			Requests:    period.Requests,
			TotalTokens: period.TotalTokens,
			TotalCost:   cost.TotalUSD,
		})
	}
	jsonHeader(w)
	json.NewEncoder(w).Encode(response)
}

func parseCompareWindows(r *http.Request, now time.Time) (time.Time, time.Time, time.Time, time.Time, error) {
	q := r.URL.Query()
	aRaw := q.Get("a")
	bRaw := q.Get("b")
	if aRaw != "" || bRaw != "" {
		if aRaw == "" || bRaw == "" {
			return time.Time{}, time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("a and b must be provided together")
		}
		aStart, aEnd, err := monthWindow(aRaw)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid a: %w", err)
		}
		bStart, bEnd, err := monthWindow(bRaw)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid b: %w", err)
		}
		return aStart, aEnd, bStart, bEnd, nil
	}

	current := monthStart(now)
	periodsRaw := q.Get("periods")
	if periodsRaw != "" {
		bucket := q.Get("bucket")
		if bucket == "" {
			bucket = "month"
		}
		if bucket != "month" {
			return time.Time{}, time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("unsupported bucket %q", bucket)
		}
		periods, err := strconv.Atoi(periodsRaw)
		if err != nil || periods < 2 {
			return time.Time{}, time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("periods must be at least 2")
		}
		aStart := current.AddDate(0, -(periods - 1), 0)
		return aStart, aStart.AddDate(0, 1, 0), current, current.AddDate(0, 1, 0), nil
	}

	last := current.AddDate(0, -1, 0)
	return last, current, current, current.AddDate(0, 1, 0), nil
}

func monthWindow(value string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01", value)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, start.AddDate(0, 1, 0), nil
}

func monthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
