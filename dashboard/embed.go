//go:build !nodashboard

// Package dashboard embeds the production build for single-binary deployment.
// Requires the dashboard to be built first (pnpm build in this directory).
//
// When the "nodashboard" build tag is set, embed_stub.go replaces this file
// so tooling can compile without the built frontend.
package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var dist embed.FS

// Handler returns an http.Handler that serves the built dashboard static files.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
