// Package webui exposes the React/Vite frontend filesystem. Production builds
// embed the generated assets into the Go binary, while ordinary development
// builds use the small fallback page until ./build.sh has been run.
package webui

import "io/fs"

// DistFS returns the embedded build rooted at dist/index.html, ready to be
// served with http.FileServerFS.
func DistFS() (fs.FS, error) {
	return embeddedDistFS()
}
