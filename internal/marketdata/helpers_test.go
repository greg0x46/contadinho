package marketdata

import (
	"context"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeClock struct {
	t     time.Time
	slept []time.Duration
}

func (c *fakeClock) now() time.Time { return c.t }

func (c *fakeClock) sleep(_ context.Context, d time.Duration) error {
	c.slept = append(c.slept, d)
	c.t = c.t.Add(d)
	return nil
}

func newTestLimiter(minInterval time.Duration) (*Limiter, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	l := newLimiter("test", minInterval, 1)
	l.now, l.sleep = clock.now, clock.sleep
	return l, clock
}

// pointAt sends a provider's client to server and strips out real waiting —
// retry backoff and limiter spacing — so a test runs instantly.
func pointAt(c *httpClient, server *httptest.Server) {
	c.client = server.Client()
	c.sleep = func(time.Duration) {}
	c.backoff = time.Millisecond
	c.limiter = newLimiter(c.provider, 0, 1)
}

// date is a calendar day the way the package stores days: UTC midnight.
func date(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
}

// fixture reads a captured provider response from testdata.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return content
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
