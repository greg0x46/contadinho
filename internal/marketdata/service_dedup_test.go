package marketdata

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

var vale3 = Instrument{Market: MarketB3, Symbol: "VALE3"}

// gatedProvider is a provider safe to call from many goroutines: it counts
// the Quote calls per instrument, announces each one on entered, and answers
// with whatever behave says — which is where a test blocks it.
type gatedProvider struct {
	behave  func(ctx context.Context, instrument Instrument, call int) (Quote, error)
	entered chan Instrument

	mu    sync.Mutex
	calls map[Instrument]int
}

func newGatedProvider(behave func(ctx context.Context, instrument Instrument, call int) (Quote, error)) *gatedProvider {
	return &gatedProvider{behave: behave, entered: make(chan Instrument, 1024), calls: map[Instrument]int{}}
}

func (g *gatedProvider) Name() string { return "gated" }

func (g *gatedProvider) Supports(market Market) bool { return market == MarketB3 }

func (g *gatedProvider) Quote(ctx context.Context, instrument Instrument) (Quote, error) {
	g.mu.Lock()
	g.calls[instrument]++
	call := g.calls[instrument]
	g.mu.Unlock()
	g.entered <- instrument
	return g.behave(ctx, instrument, call)
}

func (g *gatedProvider) History(context.Context, Instrument, time.Time, time.Time) (History, error) {
	return History{}, errors.New("gated: no history")
}

func (g *gatedProvider) callsFor(instrument Instrument) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls[instrument]
}

// brl is a good provider answer.
func brl(price string) (Quote, error) {
	return Quote{Price: decimal.RequireFromString(price), Currency: "BRL"}, nil
}

// untilReleased answers price once release is closed.
func untilReleased(release <-chan struct{}, price string) func(context.Context, Instrument, int) (Quote, error) {
	return func(context.Context, Instrument, int) (Quote, error) {
		<-release
		return brl(price)
	}
}

type quoteResult struct {
	quote Quote
	err   error
}

// askAsync runs s.Quote in a goroutine and delivers its answer on the
// returned channel.
func askAsync(ctx context.Context, s *Service, instrument Instrument) <-chan quoteResult {
	out := make(chan quoteResult, 1)
	go func() {
		quote, err := s.Quote(ctx, instrument)
		out <- quoteResult{quote, err}
	}()
	return out
}

func awaitEntered(t *testing.T, g *gatedProvider, want Instrument) {
	t.Helper()
	select {
	case got := <-g.entered:
		if got != want {
			t.Fatalf("provider asked about %s, want %s", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("provider was never asked about %s", want)
	}
}

func awaitResult(t *testing.T, what string, results <-chan quoteResult) quoteResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not return", what)
		return quoteResult{}
	}
}

// settle gives goroutines started a moment ago time to reach the point where
// they wait. It cannot make a test fail by being too short, only make it
// check less: a call arriving late meets the finished lookup's cache instead
// of the lookup under way. The provider-call counts the tests assert would
// still expose a Service that does not share lookups.
func settle() { time.Sleep(20 * time.Millisecond) }

func requireNoLookupLeft(t *testing.T, s *Service) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.inflight) != 0 {
		t.Errorf("inflight = %v, want no lookup left behind", s.inflight)
	}
}

func TestServiceSharesOneLookupAmongConcurrentQuotes(t *testing.T) {
	const callers = 50
	release := make(chan struct{})
	provider := newGatedProvider(untilReleased(release, "38.29"))
	s, now := newTestService(provider)

	results := make([]<-chan quoteResult, 0, callers)
	results = append(results, askAsync(context.Background(), s, petr4))
	awaitEntered(t, provider, petr4)
	for i := 1; i < callers; i++ {
		results = append(results, askAsync(context.Background(), s, petr4))
	}
	settle()
	close(release)

	for i, ch := range results {
		result := awaitResult(t, fmt.Sprintf("caller %d", i), ch)
		if result.err != nil || result.quote.Price.String() != "38.29" || result.quote.Provider != "gated" ||
			result.quote.Instrument != petr4 || !result.quote.At.Equal(*now) {
			t.Fatalf("caller %d got %+v, %v; want the one 38.29 quote", i, result.quote, result.err)
		}
	}
	if calls := provider.callsFor(petr4); calls != 1 {
		t.Errorf("provider asked %d times, want 1", calls)
	}
	requireNoLookupLeft(t, s)

	// The shared answer went to the cache like any other.
	if _, err := s.Quote(context.Background(), petr4); err != nil || provider.callsFor(petr4) != 1 {
		t.Errorf("later Quote: err = %v, provider asked %d times; want it served from the cache", err, provider.callsFor(petr4))
	}
}

func TestServiceLookupsOfDifferentInstrumentsDoNotWaitOnEachOther(t *testing.T) {
	release := make(chan struct{})
	provider := newGatedProvider(func(ctx context.Context, instrument Instrument, call int) (Quote, error) {
		if instrument == petr4 {
			<-release
			return brl("38.29")
		}
		return brl("60.1")
	})
	s, _ := newTestService(provider)

	slow := askAsync(context.Background(), s, petr4)
	awaitEntered(t, provider, petr4)

	fast := awaitResult(t, "VALE3 while PETR4 is in flight", askAsync(context.Background(), s, vale3))
	if fast.err != nil || fast.quote.Price.String() != "60.1" {
		t.Fatalf("VALE3 = %+v, %v; want its own 60.1 quote", fast.quote, fast.err)
	}
	select {
	case <-slow:
		t.Fatal("PETR4 returned before its provider was released")
	default:
	}

	close(release)
	if result := awaitResult(t, "PETR4", slow); result.err != nil || result.quote.Price.String() != "38.29" {
		t.Errorf("PETR4 = %+v, %v; want 38.29", result.quote, result.err)
	}
	if provider.callsFor(petr4) != 1 || provider.callsFor(vale3) != 1 {
		t.Errorf("calls = PETR4 %d, VALE3 %d; want one each", provider.callsFor(petr4), provider.callsFor(vale3))
	}
}

// A caller that gave up while leading the lookup must not hand its
// cancellation to the callers waiting on it.
func TestServiceWaiterAsksAgainWhenTheLeaderGaveUp(t *testing.T) {
	provider := newGatedProvider(func(ctx context.Context, _ Instrument, call int) (Quote, error) {
		if call == 1 {
			<-ctx.Done()
			return Quote{}, ctx.Err()
		}
		return brl("38.29")
	})
	s, _ := newTestService(provider)

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	defer cancelLeader()
	leader := askAsync(leaderCtx, s, petr4)
	awaitEntered(t, provider, petr4)
	waiter := askAsync(context.Background(), s, petr4)
	settle()
	cancelLeader()

	if result := awaitResult(t, "leader", leader); !errors.Is(result.err, context.Canceled) {
		t.Errorf("leader error = %v, want context.Canceled", result.err)
	}
	result := awaitResult(t, "waiter", waiter)
	if result.err != nil || result.quote.Price.String() != "38.29" {
		t.Fatalf("waiter = %+v, %v; want its own lookup's 38.29 quote", result.quote, result.err)
	}
	if calls := provider.callsFor(petr4); calls != 2 {
		t.Errorf("provider asked %d times, want 2: the abandoned lookup and the waiter's", calls)
	}
	requireNoLookupLeft(t, s)

	// The cancelled lookup left nothing in the cache, and the waiter's does.
	if _, err := s.Quote(context.Background(), petr4); err != nil || provider.callsFor(petr4) != 2 {
		t.Errorf("later Quote: err = %v, provider asked %d times; want it served from the cache", err, provider.callsFor(petr4))
	}
}

// Many waiters on an abandoned lookup: one of them leads the new one, the
// rest wait on it.
func TestServiceWaitersOfAnAbandonedLookupShareTheNextOne(t *testing.T) {
	const waiters = 20
	release := make(chan struct{})
	provider := newGatedProvider(func(ctx context.Context, _ Instrument, call int) (Quote, error) {
		if call == 1 {
			<-ctx.Done()
			return Quote{}, ctx.Err()
		}
		<-release
		return brl("38.29")
	})
	s, _ := newTestService(provider)

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	defer cancelLeader()
	leader := askAsync(leaderCtx, s, petr4)
	awaitEntered(t, provider, petr4)
	results := make([]<-chan quoteResult, waiters)
	for i := range results {
		results[i] = askAsync(context.Background(), s, petr4)
	}
	settle()
	cancelLeader()
	awaitResult(t, "leader", leader)
	awaitEntered(t, provider, petr4) // the second lookup, led by one of the waiters
	settle()
	close(release)

	for i, ch := range results {
		if result := awaitResult(t, fmt.Sprintf("waiter %d", i), ch); result.err != nil || result.quote.Price.String() != "38.29" {
			t.Fatalf("waiter %d = %+v, %v; want the 38.29 quote", i, result.quote, result.err)
		}
	}
	if calls := provider.callsFor(petr4); calls != 2 {
		t.Errorf("provider asked %d times, want 2", calls)
	}
	requireNoLookupLeft(t, s)
}

// A waiter that gives up leaves with its own context's error and costs the
// lookup, and the callers still waiting on it, nothing.
func TestServiceCancelledWaiterLeavesTheOthersAlone(t *testing.T) {
	release := make(chan struct{})
	provider := newGatedProvider(untilReleased(release, "38.29"))
	s, _ := newTestService(provider)

	leader := askAsync(context.Background(), s, petr4)
	awaitEntered(t, provider, petr4)
	quitterCtx, cancelQuitter := context.WithCancel(context.Background())
	defer cancelQuitter()
	quitter := askAsync(quitterCtx, s, petr4)
	patient := askAsync(context.Background(), s, petr4)
	settle()

	cancelQuitter()
	// It returns while the lookup is still blocked: it did not wait for it.
	if result := awaitResult(t, "cancelled waiter", quitter); !errors.Is(result.err, context.Canceled) {
		t.Errorf("cancelled waiter error = %v, want context.Canceled", result.err)
	}
	select {
	case <-leader:
		t.Fatal("leader returned before its provider was released")
	case <-patient:
		t.Fatal("waiter returned before the provider was released")
	default:
	}

	close(release)
	for name, ch := range map[string]<-chan quoteResult{"leader": leader, "patient waiter": patient} {
		if result := awaitResult(t, name, ch); result.err != nil || result.quote.Price.String() != "38.29" {
			t.Errorf("%s = %+v, %v; want the 38.29 quote", name, result.quote, result.err)
		}
	}
	if calls := provider.callsFor(petr4); calls != 1 {
		t.Errorf("provider asked %d times, want 1", calls)
	}
	requireNoLookupLeft(t, s)
}

// A caller that is already cancelled does not start, or join, a lookup it
// cannot wait for.
func TestServiceWaiterCancelledBeforeAskingReturnsItsOwnError(t *testing.T) {
	release := make(chan struct{})
	provider := newGatedProvider(untilReleased(release, "38.29"))
	s, _ := newTestService(provider)

	leader := askAsync(context.Background(), s, petr4)
	awaitEntered(t, provider, petr4)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result := awaitResult(t, "cancelled caller", askAsync(ctx, s, petr4)); !errors.Is(result.err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", result.err)
	}

	close(release)
	if result := awaitResult(t, "leader", leader); result.err != nil {
		t.Errorf("leader error = %v", result.err)
	}
	if calls := provider.callsFor(petr4); calls != 1 {
		t.Errorf("provider asked %d times, want 1", calls)
	}
}

// A provider failure is the answer for everyone who was waiting on that
// lookup, and for nobody after it.
func TestServiceSharedFailureIsNotCached(t *testing.T) {
	const callers = 20
	release := make(chan struct{})
	provider := newGatedProvider(func(_ context.Context, _ Instrument, call int) (Quote, error) {
		if call == 1 {
			<-release
			return Quote{}, fmt.Errorf("gated: %w", ErrUnavailable)
		}
		return brl("38.29")
	})
	s, _ := newTestService(provider)

	results := make([]<-chan quoteResult, 0, callers)
	results = append(results, askAsync(context.Background(), s, petr4))
	awaitEntered(t, provider, petr4)
	for i := 1; i < callers; i++ {
		results = append(results, askAsync(context.Background(), s, petr4))
	}
	settle()
	close(release)

	for i, ch := range results {
		if result := awaitResult(t, fmt.Sprintf("caller %d", i), ch); !errors.Is(result.err, ErrUnavailable) {
			t.Fatalf("caller %d error = %v, want the lookup's ErrUnavailable", i, result.err)
		}
	}
	if calls := provider.callsFor(petr4); calls != 1 {
		t.Errorf("provider asked %d times, want 1 for all the callers that were waiting", calls)
	}
	requireNoLookupLeft(t, s)

	quote, err := s.Quote(context.Background(), petr4)
	if err != nil || quote.Price.String() != "38.29" {
		t.Fatalf("next Quote = %+v, %v; want the provider asked again", quote, err)
	}
	if calls := provider.callsFor(petr4); calls != 2 {
		t.Errorf("provider asked %d times, want 2: the failure must not be remembered", calls)
	}
}

// A provider that panics must not leave its waiters stuck on a lookup that
// will never finish, nor the instrument unqueryable.
func TestServiceWaiterAsksAgainWhenTheLeaderPanicked(t *testing.T) {
	release := make(chan struct{})
	provider := newGatedProvider(func(_ context.Context, _ Instrument, call int) (Quote, error) {
		if call == 1 {
			<-release
			panic("gated: boom")
		}
		return brl("38.29")
	})
	s, _ := newTestService(provider)

	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		s.Quote(context.Background(), petr4)
	}()
	awaitEntered(t, provider, petr4)
	waiter := askAsync(context.Background(), s, petr4)
	settle()
	close(release)

	select {
	case got := <-panicked:
		if got == nil {
			t.Error("leader returned normally, want the provider's panic to reach it")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("leader did not return")
	}
	if result := awaitResult(t, "waiter", waiter); result.err != nil || result.quote.Price.String() != "38.29" {
		t.Fatalf("waiter = %+v, %v; want its own lookup's 38.29 quote", result.quote, result.err)
	}
	requireNoLookupLeft(t, s)
}

// Quote calls that do not overlap behave as they always did: each one that
// misses the cache asks the providers.
func TestServiceSequentialQuotesStillAskTheProvidersAfterTheTTL(t *testing.T) {
	provider := newGatedProvider(func(context.Context, Instrument, int) (Quote, error) { return brl("38.29") })
	s, now := newTestService(provider)

	for i := 1; i <= 3; i++ {
		if _, err := s.Quote(context.Background(), petr4); err != nil {
			t.Fatalf("Quote %d: %v", i, err)
		}
		*now = now.Add(quoteTTL)
		if calls := provider.callsFor(petr4); calls != i {
			t.Fatalf("after Quote %d the provider was asked %d times, want %d", i, calls, i)
		}
	}
	requireNoLookupLeft(t, s)
}
