// Package httpapi is the net/http layer: it decodes requests, calls the
// domain packages (money/categories/transactions), and encodes responses in
// the same JSON shape frontend/src/api/contracts.ts expects, so the existing
// React frontend needs no changes to talk to this server.
package httpapi

import (
	"encoding/json"
	"net/http"
)

// Problem mirrors web/schemas.py's ProblemResponse (RFC 7807-flavored error
// body), served with the application/problem+json content type the
// frontend's client.ts specifically checks for.
type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func writeProblem(w http.ResponseWriter, status int, kind, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{
		Type:   "/problems/" + kind,
		Title:  title,
		Status: status,
		Detail: detail,
	})
}
