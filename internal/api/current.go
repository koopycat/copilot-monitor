package api

import (
	"encoding/json"
	"net/http"
	"time"

	"copilot-monitoring/internal/catalog"
	costcalc "copilot-monitoring/internal/cost"
)

type currentSessionResponse struct {
	Session *currentSessionInfo   `json:"session"`
	Models  []currentSessionModel `json:"models"`
}

type currentSessionInfo struct {
	ID            int64     `json:"id"`
	StartedAt     time.Time `json:"started_at"`
	LastRequestAt time.Time `json:"last_request_at"`
	Project       string    `json:"project"`
	RequestCount  int       `json:"request_count"`
	TokenCount    int       `json:"token_count"`
	Cost          float64   `json:"cost"`
	Status        string    `json:"status"`
	Active        bool      `json:"active"`
}

type currentSessionModel struct {
	Model    string  `json:"model"`
	Endpoint string  `json:"endpoint"`
	Requests int     `json:"requests"`
	Tokens   int     `json:"tokens"`
	Cost     float64 `json:"cost"`
}

func (h *Handler) handleCurrentSession(w http.ResponseWriter, r *http.Request) {
	if err := h.db.RebuildSessions(r.Context(), 30*time.Minute); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	current, err := h.db.CurrentSession(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := currentSessionResponse{Models: []currentSessionModel{}}
	if current != nil {
		cat, err := catalog.LoadDefault()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cost := costcalc.Calculate(current.Models, cat)
		response.Session = &currentSessionInfo{
			ID:            current.ID,
			StartedAt:     current.StartedAt,
			LastRequestAt: current.LastRequestAt,
			Project:       current.Project,
			RequestCount:  current.RequestCount,
			TokenCount:    current.TokenCount,
			Cost:          cost.TotalUSD,
			Status:        current.Status,
			Active:        current.Active,
		}
		for _, row := range cost.Rows {
			response.Models = append(response.Models, currentSessionModel{
				Model:    row.Model,
				Endpoint: row.Endpoint,
				Requests: row.Requests,
				Tokens:   row.TotalTokens,
				Cost:     row.TotalUSD,
			})
		}
	}
	jsonHeader(w)
	json.NewEncoder(w).Encode(response)
}
