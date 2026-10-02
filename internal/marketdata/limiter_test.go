package marketdata

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func mustAcquire(t *testing.T, l *Limiter) {
	t.Helper()
	release, err := l.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
}

func TestLimiterSpacesRequestsByTheMinimumInterval(t *testing.T) {
	l, clock := newTestLimiter(3 * time.Second)

	mustAcquire(t, l) // starts immediately
	mustAcquire(t, l) // has to wait the interval
	mustAcquire(t, l)

	if len(clock.slept) != 2 || clock.slept[0] != 3*time.Second || clock.slept[1] != 3*time.Second {
		t.Fatalf("slept = %v, want two waits of 3s", clock.slept)
	}
}

func TestLimiterHoldsEverythingBackWhileAProviderAsksForAPause(t *testing.T) {
	l, clock := newTestLimiter(0)

	header := http.Header{"Retry-After": []string{"20"}}
	l.Observe(http.StatusTooManyRequests, header)
	mustAcquire(t, l)

	if len(clock.slept) != 1 || clock.slept[0] != 20*time.Second {
		t.Fatalf("slept = %v, want one wait of the Retry-After (20s)", clock.slept)
	}
}

func TestLimiterFailsFastWhenThePauseIsLongerThanWorthWaiting(t *testing.T) {
	l, clock := newTestLimiter(0)

	l.Observe(http.StatusTooManyRequests, http.Header{"Retry-After": []string{"300"}})
	_, err := l.Acquire(context.Background())

	var limited *RateLimitedError
	if !errors.As(err, &limited) {
		t.Fatalf("Acquire error = %v, want a *RateLimitedError", err)
	}
	if limited.RetryAfter != 300*time.Second || limited.Provider != "test" {
		t.Errorf("limited = %+v, want 300s for provider test", limited)
	}
	if len(clock.slept) != 0 {
		t.Errorf("slept = %v, a request that cannot start must not sleep", clock.slept)
	}
}

func TestLimiterPausesWhenTheRateLimitWindowIsSpent(t *testing.T) {
	l, clock := newTestLimiter(0)

	// brapi's draft-style headers: nothing remaining, window resets in 12s.
	l.Observe(http.StatusOK, http.Header{"Ratelimit-Remaining": []string{"0"}, "Ratelimit-Reset": []string{"12"}})
	mustAcquire(t, l)

	if len(clock.slept) != 1 || clock.slept[0] != 12*time.Second {
		t.Fatalf("slept = %v, want one wait of the window reset (12s)", clock.slept)
	}
}

func TestLimiterIgnoresAWindowThatStillHasBudget(t *testing.T) {
	l, clock := newTestLimiter(0)

	l.Observe(http.StatusOK, http.Header{"Ratelimit-Remaining": []string{"7"}, "Ratelimit-Reset": []string{"40"}})
	mustAcquire(t, l)

	if len(clock.slept) != 0 {
		t.Fatalf("slept = %v, a window with budget left must not pause anything", clock.slept)
	}
}

func TestLimiterReadsAnEpochReset(t *testing.T) {
	l, clock := newTestLimiter(0)

	reset := clock.t.Add(9 * time.Second).Unix()
	l.Observe(http.StatusOK, http.Header{
		"X-Ratelimit-Remaining": []string{"0"},
		"X-Ratelimit-Reset":     []string{strconv.FormatInt(reset, 10)},
	})
	mustAcquire(t, l)

	if len(clock.slept) != 1 || clock.slept[0] != 9*time.Second {
		t.Fatalf("slept = %v, want one wait of 9s", clock.slept)
	}
}

func TestLimiterSerializesRequestsToTheProvidersConcurrency(t *testing.T) {
	l, _ := newTestLimiter(0)

	release, err := l.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := l.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second Acquire = %v, want it to block until the first is released", err)
	}
	release()
	mustAcquire(t, l)
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"seconds", "30", 30 * time.Second},
		{"http date", now.Add(45 * time.Second).Format(http.TimeFormat), 45 * time.Second},
		{"date in the past", now.Add(-time.Minute).Format(http.TimeFormat), 0},
		{"negative", "-5", 0},
		{"garbage", "soon", 0},
		{"absent", "", 0},
	} {
		header := http.Header{}
		if tc.value != "" {
			header.Set("Retry-After", tc.value)
		}
		if got := parseRetryAfter(header, now); got != tc.want {
			t.Errorf("%s: parseRetryAfter(%q) = %v, want %v", tc.name, tc.value, got, tc.want)
		}
	}
}
