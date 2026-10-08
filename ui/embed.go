// Package ui embeds the exported Expo web app (ui/dist) into the binary.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the SPA: real files as-is, any other HTML navigation gets index.html.
func Handler() http.Handler {
	sub, _ := fs.Sub(dist, "dist")
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return http.NotFoundHandler() // UI not built
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if fi, err := fs.Stat(sub, p); err == nil && !fi.IsDir() { // no directory listings
				if strings.HasPrefix(p, "_expo/static/") { // content-hashed by expo export
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			// A missing bundle (a tab still running the previous build) is a 404, not index.html served as JS.
			if strings.HasPrefix(p, "_expo/") || strings.HasPrefix(p, "assets/") {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}
