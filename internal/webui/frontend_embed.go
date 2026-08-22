//go:build embedded_frontend

package webui

import (
	"embed"
	"io/fs"
)

// The production build populates this directory before invoking go build.
//
//go:embed dist
var distFS embed.FS

func embeddedDistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
