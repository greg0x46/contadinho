package quotes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpClient is the shared shape both connectors retry through: a plain
// http.Client with a fixed timeout — this codebase never uses
// context.WithTimeout (see internal/pluggy/client.go for the same
// convention) — plus linear backoff on 429/5xx. A quote lookup has none of
// internal/pluggy Adapter.read's auth flow, pagination or raw_imports
// persistence, so this is a fraction of its size.
type httpClient struct {
	client      *http.Client
	maxAttempts int
	backoff     time.Duration
	sleep       func(time.Duration)
}

func newHTTPClient() *httpClient {
	return &httpClient{
		client:      &http.Client{Timeout: 10 * time.Second},
		maxAttempts: 3,
		backoff:     500 * time.Millisecond,
		sleep:       time.Sleep,
	}
}

// httpStatusError is a non-2xx response that got all the way back from the
// wire. Exported fields let a connector distinguish "not found"/"needs a
// token" (4xx, not retried) from a rate limit or outage (429/5xx, retried
// then surfaced only once attempts are exhausted).
type httpStatusError struct {
	StatusCode int
	Body       string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("status %d: %s", e.StatusCode, e.Body)
}

// getJSON issues a GET, retrying on 429/5xx with linear backoff, and decodes
// the response body with UseNumber() so a caller can turn a price into
// decimal.Decimal without ever passing it through float64.
func (c *httpClient) getJSON(ctx context.Context, url string, headers map[string]string) (map[string]any, error) {
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.maxAttempts {
				c.sleep(time.Duration(attempt) * c.backoff)
				continue
			}
			return nil, fmt.Errorf("request failed: %w", err)
		}

		content, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = &httpStatusError{StatusCode: resp.StatusCode, Body: string(content)}
			if attempt < c.maxAttempts {
				c.sleep(time.Duration(attempt) * c.backoff)
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode >= 400 {
			return nil, &httpStatusError{StatusCode: resp.StatusCode, Body: string(content)}
		}

		dec := json.NewDecoder(bytes.NewReader(content))
		dec.UseNumber()
		var decoded map[string]any
		if err := dec.Decode(&decoded); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		return decoded, nil
	}
	return nil, lastErr
}
