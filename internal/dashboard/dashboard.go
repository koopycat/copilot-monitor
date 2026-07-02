package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html app.js chart.js
var dashboardFS embed.FS

func DashboardHandler() http.Handler {
	sub, err := fs.Sub(dashboardFS, ".")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
