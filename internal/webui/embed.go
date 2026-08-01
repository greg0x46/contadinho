// Package webui embeds the React/Vite frontend's production build (copied
// from ~/code/contadinho/frontend/dist) into the Go binary, so the server
// needs no separate static-file deployment step.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// DistFS returns the embedded build rooted at dist/index.html, ready to be
// served with http.FileServerFS.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
