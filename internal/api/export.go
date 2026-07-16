package api

import (
	"fmt"
	"net/http"
	"strings"
)

func (h *Handler) handleExport(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.ExportRequests(r.Context(), parseSinceParam(r), parseUntilParam(r), parseUpstreamParam(r))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=export.csv")
	w.Write([]byte("ts,endpoint,model,status,latency_ms,prompt_tokens,cached_input_tokens,cache_write_tokens,completion_tokens,total_tokens,project,headroom_proxied\n"))
	for _, row := range rows {
		fmt.Fprintf(w, "%s,%s,%s,%d,%d,%d,%d,%d,%d,%d,%s,%t\n",
			row.Timestamp, row.Endpoint, csvEscape(row.Model), row.Status, row.LatencyMS,
			row.PromptTokens, row.CachedInputTokens, row.CacheWriteTokens,
			row.CompletionTokens, row.TotalTokens, csvEscape(row.Project),
			row.HeadroomProxied)
	}
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
