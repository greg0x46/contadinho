package marketdata

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// jsonOfSize is a JSON object exactly size bytes long.
func jsonOfSize(size int) []byte {
	const overhead = len(`{"pad":""}`)
	return []byte(`{"pad":"` + strings.Repeat("x", size-overhead) + `"}`)
}

func TestHTTPClientDefaultsToTheResponseCap(t *testing.T) {
	limiter, _ := newTestLimiter(0)
	if client := newHTTPClient("test", limiter); client.maxBytes != maxResponseBytes {
		t.Errorf("maxBytes = %d, want %d", client.maxBytes, maxResponseBytes)
	}
}

// A body over the cap is an outage, not worth asking for again, and leaves
// the limiter's slot free for the next request.
func TestHTTPClientRefusesAResponseOverTheCap(t *testing.T) {
	const limit = 1 << 10
	attempts := 0
	client, url, backoffs, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Write(jsonOfSize(limit + 1))
			return
		}
		w.Write([]byte(`{"ok":1}`))
	})
	client.maxBytes = limit

	_, err := getMap(client, url)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
	if want := fmt.Sprintf("larger than %d bytes", limit); !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to name the limit (%q)", err, want)
	}
	if attempts != 1 || len(*backoffs) != 0 {
		t.Errorf("attempts = %d, backoffs = %v; want one request, no retry", attempts, *backoffs)
	}

	// The limiter admits one request at a time: this one only gets through
	// if the oversized one released its slot.
	done := make(chan error, 1)
	go func() {
		_, err := getMap(client, url)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("request after the oversized one: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request after the oversized one never started: the limiter slot was not released")
	}
}

func TestHTTPClientDecodesAResponseJustUnderAndAtTheCap(t *testing.T) {
	const limit = 1 << 10
	for name, size := range map[string]int{"just under": limit - 1, "exactly at": limit} {
		client, url, _, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write(jsonOfSize(size))
		})
		client.maxBytes = limit

		payload, err := getMap(client, url)
		if err != nil {
			t.Errorf("%s the cap: %v", name, err)
			continue
		}
		if pad, _ := payload["pad"].(string); len(pad) != size-len(`{"pad":""}`) {
			t.Errorf("%s the cap: pad has %d bytes, want the body decoded whole", name, len(pad))
		}
	}
}

// With no Content-Length the size is only known by reading: the cap has to
// hold for a body that streams on (chunked) past it too.
func TestHTTPClientCapsAStreamedResponse(t *testing.T) {
	const limit = 1 << 10
	client, url, _, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Write([]byte(`{"pad":"`))
		flusher.Flush()
		for range 10 {
			w.Write([]byte(strings.Repeat("x", limit/2)))
			flusher.Flush()
		}
		w.Write([]byte(`"}`))
	})
	client.maxBytes = limit

	if _, err := getMap(client, url); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}
