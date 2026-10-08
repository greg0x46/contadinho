package marketdata

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// maxInlineWait is the longest a request waits for a provider to be ready
// before giving up with a RateLimitedError instead. Anything longer is a
// pause the provider asked for (a minute-long cooldown, a quota reset), and
// holding a request open that long only stalls every asset queued behind it;
// the caller defers the rest of that provider's work to the next run instead.
const maxInlineWait = 30 * time.Second

// defaultPenalty is how long a provider is left alone after a 429 that did not
// say how long to wait.
const defaultPenalty = 60 * time.Second

// Limiter spaces and serializes the requests to one provider, and stops them
// altogether while the provider asks for a pause. It is shared by every
// client of that provider for the life of the process: the daily run, the
// backfill worker and a symbol lookup all draw from the same budget, because
// the provider counts them together.
type Limiter struct {
	name        string
	minInterval time.Duration
	maxWait     time.Duration
	slots       chan struct{}

	mu          sync.Mutex
	next        time.Time
	pausedUntil time.Time

	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

func newLimiter(name string, minInterval time.Duration, concurrency int) *Limiter {
	return &Limiter{
		name: name, minInterval: minInterval, maxWait: maxInlineWait,
		slots: make(chan struct{}, concurrency),
		now:   time.Now,
		sleep: sleepContext,
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Acquire blocks until a request may start — spaced from the previous one,
// outside any pause, and within the provider's concurrency — and returns the
// release to call when the response is in. When the wait would exceed
// maxInlineWait it fails fast with a RateLimitedError.
func (l *Limiter) Acquire(ctx context.Context) (func(), error) {
	for {
		l.mu.Lock()
		now := l.now()
		ready := l.next
		if l.pausedUntil.After(ready) {
			ready = l.pausedUntil
		}
		if !ready.After(now) {
			l.next = now.Add(l.minInterval)
			l.mu.Unlock()
			break
		}
		wait := ready.Sub(now)
		l.mu.Unlock()
		if wait > l.maxWait {
			return nil, &RateLimitedError{Provider: l.name, RetryAfter: wait}
		}
		if err := l.sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
	select {
	case l.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return func() { <-l.slots }, nil
}

// Pause holds every request to the provider back for d.
func (l *Limiter) Pause(d time.Duration) {
	if d <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if until := l.now().Add(d); until.After(l.pausedUntil) {
		l.pausedUntil = until
	}
}

// Observe reads what a response says about the provider's budget and pauses
// the provider when it is spent: a Retry-After on a 429, or an exhausted
// RateLimit-Remaining window (brapi sends the RateLimit-* family on every
// response, so the pause starts before the 429 would).
func (l *Limiter) Observe(status int, header http.Header) {
	if status == http.StatusTooManyRequests {
		if d := parseRetryAfter(header, l.now()); d > 0 {
			l.Pause(d)
		}
		return
	}
	remaining, hasRemaining := headerInt(header, "RateLimit-Remaining", "X-RateLimit-Remaining")
	if hasRemaining && remaining <= 0 {
		if reset, ok := headerInt(header, "RateLimit-Reset", "X-RateLimit-Reset"); ok {
			l.Pause(resetToDuration(reset, l.now()))
		} else {
			l.Pause(defaultPenalty)
		}
	}
}

// parseRetryAfter reads Retry-After as delta-seconds or an HTTP date. Zero
// means the header is absent or unreadable.
func parseRetryAfter(header http.Header, now time.Time) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		if d := at.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

func headerInt(header http.Header, names ...string) (int, bool) {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			if n, err := strconv.Atoi(value); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

// resetToDuration reads a RateLimit-Reset value: delta-seconds in the IETF
// draft brapi follows, or an epoch timestamp in the X-RateLimit-Reset
// dialect.
func resetToDuration(reset int, now time.Time) time.Duration {
	if reset > 1_000_000_000 {
		if d := time.Unix(int64(reset), 0).Sub(now); d > 0 {
			return d
		}
		return 0
	}
	if reset <= 0 {
		return time.Second
	}
	return time.Duration(reset) * time.Second
}

// The per-provider budgets, deliberately below what each provider documents
// so the app never competes with itself for the last request of a window.
//
//   - Yahoo's chart endpoint is unofficial and publishes no limit at all; what
//     it does is answer 429 to clients it dislikes. One request a second,
//     never two at once, is far below anything an interactive page makes, and
//     a 429 pauses the provider for its Retry-After like everywhere else.
//   - CoinGecko's keyless API is rate limited per IP and shared with every
//     other keyless user behind that address; it publishes no number for it
//     (the keyed Demo plan is 100 calls/min). One call every six seconds is
//     ten a minute, and a 429 pauses the provider for its Retry-After.
//   - brapi without a token allows 20 requests/min per IP (RateLimit-Limit:
//     20 on every response). One call every 3.2s is under 19 a minute. With a
//     token there is no per-minute cap, only the plan's monthly quota and an
//     account-wide concurrency of 1 on the Free plan, so requests are
//     serialized with a short gap.
var (
	yahooLimiter          = newLimiter(ProviderYahoo, time.Second, 1)
	coingeckoLimiter      = newLimiter(ProviderCoinGecko, 6*time.Second, 1)
	brapiAnonymousLimiter = newLimiter(ProviderBrapi, 3200*time.Millisecond, 1)
	brapiTokenLimiter     = newLimiter(ProviderBrapi, 300*time.Millisecond, 1)
)
