package server

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"regexp"
	"sort"
)

// The browser-mirror web UI (view-only) is embedded so a single binary serves it
// with no build-time node_modules dependency. The vendored xterm.js files are
// refreshed by mobileapp/scripts/gen-web-assets.mjs.
//
//go:embed web
var webFS embed.FS

// assetTag is a short content hash of the embedded web assets, appended as a
// ?v= query to the asset URLs inside index.html. A new build → new tag → new
// cache key, so a freshly-updated serve is never masked by a stale .js/.css
// sitting in a CDN/tunnel edge cache: a Cloudflare tunnel applies a multi-hour
// default Browser-Cache-TTL to .js/.css and a client can't bust it, so after
// `gtmux update` the browser kept loading the OLD app.js ("no chat mode").
// index.html itself is served uncached and edges treat it as dynamic, so the new
// tag — and thus the new assets — propagate on the next reload.
var assetTag = hashWeb()

func hashWeb() string {
	h := sha256.New()
	var names []string
	_ = fs.WalkDir(webFS, "web", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, p)
		}
		return nil
	})
	sort.Strings(names)
	for _, n := range names {
		b, _ := webFS.ReadFile(n)
		h.Write([]byte(n))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// assetRefRe matches relative href=/src= asset URLs in index.html (no scheme — the
// `[^":]` excludes absolute https:// URLs). Used to append the cache-busting tag.
var assetRefRe = regexp.MustCompile(`(href|src)="([^":]+\.(?:css|js))"`)

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
	fileSrv := http.FileServer(http.FS(sub))

	// Pre-render index.html once with cache-busted asset refs.
	index, _ := fs.ReadFile(sub, "index.html")
	index = assetRefRe.ReplaceAll(index, []byte(`$1="$2?v=`+assetTag+`"`))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// no-cache so neither the browser nor a CDN/tunnel edge serves a stale copy
		// after an update (belt-and-suspenders with the ?v= tag above).
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if p := r.URL.Path; (p == "/" || p == "/index.html") && len(index) > 0 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(index)
			return
		}
		fileSrv.ServeHTTP(w, r)
	})
}
