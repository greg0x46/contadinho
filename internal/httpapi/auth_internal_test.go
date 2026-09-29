package httpapi

import "testing"

func TestOriginAllowed(t *testing.T) {
	cases := []struct {
		publicURL, origin string
		want              bool
	}{
		{"http://127.0.0.1:5173", "http://127.0.0.1:5173", true},
		{"http://127.0.0.1:5173", "http://localhost:5173", true},
		{"http://localhost:5173", "http://127.0.0.1:5173", true},
		{"http://127.0.0.1:5173", "http://[::1]:5173", true},
		{"http://127.0.0.1:5173", "http://127.0.0.1:5174", false},
		{"http://127.0.0.1:5173", "http://evil.example:5173", false},
		{"https://finance.example", "https://finance.example", true},
		{"https://finance.example", "http://localhost:5173", false},
		{"http://127.0.0.1:5173", "", false},
	}
	for _, c := range cases {
		if got := originAllowed(c.publicURL, c.origin); got != c.want {
			t.Errorf("originAllowed(%q, %q) = %v, want %v", c.publicURL, c.origin, got, c.want)
		}
	}
}
