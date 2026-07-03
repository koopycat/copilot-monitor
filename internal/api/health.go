package api

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonHeader(w)
	json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
	})
}
