//go:build nodashboard

// Package dashboard provides a stub Handler when built with the
// "nodashboard" build tag. This lets Go tooling (go build, go vet,
// staticcheck, go test) run without first building the frontend bundle,
// because the production embed (embed.go) requires dashboard/dist to exist.
//
// Production/release binaries MUST NOT use this tag — they build the
// dashboard first and embed the real assets via embed.go.
package dashboard

import "net/http"

// Handler returns a placeholder handler when the dashboard assets are not
// embedded (compiled with -tags nodashboard). Useful for CI compile/test
// jobs that do not need the UI.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "dashboard not built (binary compiled with -tags nodashboard)", http.StatusNotFound)
	})
}
