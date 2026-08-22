//go:build !embedded_frontend

package webui

import (
	"embed"
	"io/fs"
)

// This keeps ordinary Go builds and tests usable without requiring generated
// frontend assets in the repository.
//
//go:embed fallback
var fallbackFS embed.FS

func embeddedDistFS() (fs.FS, error) {
	return fs.Sub(fallbackFS, "fallback")
}
