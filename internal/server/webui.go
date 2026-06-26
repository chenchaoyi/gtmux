package server

import (
	"embed"
	"io/fs"
	"net/http"
)

// The browser-mirror web UI (view-only) is embedded so a single binary serves it
// with no build-time node_modules dependency. The vendored xterm.js files are
// refreshed by mobileapp/scripts/gen-web-assets.mjs.
//
//go:embed web
var webFS embed.FS

// webHandler serves the embedded browser-mirror UI at "/" (and its static assets).
// It is UNAUTHENTICATED on purpose: the page itself must load before it can pair
// and then authenticate every /api/* call with a per-device token. The more
// specific /api/* routes are registered first, so this only ever sees non-API
// paths.
func webHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}
