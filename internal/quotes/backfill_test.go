package quotes

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/greg0x46/julius/internal/investments"
	"github.com/greg0x46/julius/internal/marketdata"
)

// historyCall is one FetchHistory request a fake connector received.
type historyCall struct {
	symbol   string
	from, to time.Time
}

// fakeHistoryConnector prices every day it is asked about at 100 plus the
// day's offset from the start of the range, counts what it was asked, and can
// be told to resolve tickers or to ask for a pause.
type fakeHistoryConnector struct {
	provider   string
	spot       map[string]decimal.Decimal
	calls      []historyCall
	err        error
	servedFrom *time.Time // when set, the answer reaches back no further than this
	market     marketdata.Market
}

func (f *fakeHistoryConnector) Name() string {
	if f.provider != "" {
		return f.provider
	}
	return marketdata.ProviderYahoo
}
func (f *fakeHistoryConnector) Supports(market marketdata.Market) bool {
	return f.market == "" || market == f.market
}
func (f *fakeHistoryConnector) Quote(_ context.Context, instrument marketdata.Instrument) (marketdata.Quote, error) {
	symbol := instrument.Symbol
	if f.err != nil {
		return marketdata.Quote{}, f.err
	}
	price, ok := f.spot[symbol]
	if !ok {
		return marketdata.Quote{}, fmt.Errorf("fake: no spot price for %q", symbol)
	}
	return marketdata.Quote{Price: price, Currency: "BRL"}, nil
}

func (f *fakeHistoryConnector) History(_ context.Context, instrument marketdata.Instrument, from, to time.Time) (marketdata.History, error) {
	symbol := instrument.Symbol
	f.calls = append(f.calls, historyCall{symbol: symbol, from: from, to: to})
	if f.err != nil {
		return marketdata.History{}, f.err
	}
	start := from
	if f.servedFrom != nil && f.servedFrom.After(start) {
		start = *f.servedFrom
	}
	var prices []marketdata.DailyPrice
	for day := start; !day.After(to); day = day.AddDate(0, 0, 1) {
		prices = append(prices, marketdata.DailyPrice{Day: day, Price: decimal.NewFromInt(100 + int64(day.Sub(from).Hours()/24))})
	}
	return marketdata.History{Prices: prices, CoveredFrom: start, Currency: "BRL"}, nil
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// mustCreateTickerAsset is what a person does: a ticker and a source, no symbol.
func mustCreateTickerAsset(t *testing.T, ctx context.Context, conn *sql.DB, name, ticker, assetType, source string) investments.Asset {
	t.Helper()
	asset, err := investments.CreateAsset(ctx, conn, investments.AssetInput{
		Name: name, Ticker: &ticker, AssetType: assetType, CurrencyCode: "BRL", QuoteSource: &source,
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	return asset
}

// mustOpenPosition buys 10 units of asset on openedOn in a fresh account.
func mustOpenPosition(t *testing.T, ctx context.Context, conn *sql.DB, accountName string, asset investments.Asset, openedOn time.Time) investments.Position {
	t.Helper()
	account, err := investments.CreateAccount(ctx, conn, investments.AccountInput{Name: accountName})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	position, err := investments.CreatePosition(ctx, conn, investments.PositionInput{
		AccountID: account.ID, AssetID: &asset.ID, Name: asset.Name, AssetType: asset.AssetType,
		InitialQuantity: decimal.NewFromInt(10), InitialUnitCost: decimal.NewFromInt(1), OccurredOn: openedOn,
	})
	if err != nil {
		t.Fatalf("CreatePosition: %v", err)
	}
	return position
}

func quotesOf(t *testing.T, ctx context.Context, conn *sql.DB, assetID string) []investments.AssetQuote {
	t.Helper()
	quotes, err := investments.ListAssetQuotes(ctx, conn, assetID, nil)
	if err != nil {
		t.Fatalf("ListAssetQuotes: %v", err)
	}
	return quotes
}

func coverageOf(t *testing.T, ctx context.Context, conn *sql.DB, assetID string) *investments.QuoteCoverage {
	t.Helper()
	coverage, err := investments.GetQuoteCoverage(ctx, conn, assetID)
	if err != nil {
		t.Fatalf("GetQuoteCoverage: %v", err)
	}
	return coverage
}

func mustRefreshHistory(t *testing.T, ctx context.Context, conn *sql.DB, service *marketdata.Service, today time.Time) HistorySummary {
	t.Helper()
	summary, err := RefreshHistory(ctx, conn, service, today)
	if err != nil {
		t.Fatalf("RefreshHistory: %v", err)
	}
	return summary
}

func TestMissingRange(t *testing.T) {
	held := date(2026, 9, 10)
	yesterday := date(2026, 9, 30)
	covered := func(from, to time.Time) *investments.QuoteCoverage {
		return &investments.QuoteCoverage{From: from, To: to}
	}
	for _, tc := range []struct {
		name         string
		held         time.Time
		covered      *investments.QuoteCoverage
		wantFrom     time.Time
		wantTo       time.Time
		wantNeedsAsk bool
	}{
		{"never asked", held, nil, held, yesterday, true},
		{"fully covered", held, covered(date(2026, 9, 10), date(2026, 9, 30)), time.Time{}, time.Time{}, false},
		{"covered more widely than needed", held, covered(date(2026, 8, 1), date(2026, 9, 30)), time.Time{}, time.Time{}, false},
		{"an earlier purchase extends it backwards", date(2026, 9, 1), covered(date(2026, 9, 10), date(2026, 9, 30)), date(2026, 9, 1), date(2026, 9, 9), true},
		{"missed days extend it forwards", held, covered(date(2026, 9, 10), date(2026, 9, 26)), date(2026, 9, 27), yesterday, true},
		{"both ends at once is one range", date(2026, 9, 1), covered(date(2026, 9, 10), date(2026, 9, 26)), date(2026, 9, 1), yesterday, true},
		{"bought today", date(2026, 10, 1), nil, time.Time{}, time.Time{}, false},
	} {
		from, to, needed := missingRange(tc.held, yesterday, tc.covered)
		if needed != tc.wantNeedsAsk || (needed && (!from.Equal(tc.wantFrom) || !to.Equal(tc.wantTo))) {
			t.Errorf("%s: missingRange = %s..%s needed=%v, want %s..%s needed=%v", tc.name,
				from.Format("2006-01-02"), to.Format("2006-01-02"), needed,
				tc.wantFrom.Format("2006-01-02"), tc.wantTo.Format("2006-01-02"), tc.wantNeedsAsk)
		}
	}
}

// The scenario that motivated all of this: a position registered with a date
// in the past gets its prices for those dates, once.
func TestRefreshHistoryBackfillsAPastDatedPositionOnce(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 20))
	fake := &fakeHistoryConnector{}
	service := marketdata.New(fake)
	today := date(2026, 10, 1)

	summary := mustRefreshHistory(t, ctx, conn, service, today)

	if summary.HistoriesFetched != 1 || summary.PricesStored != 11 || summary.Failures != 0 {
		t.Fatalf("summary = %+v, want 1 history of 11 days", summary)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("calls = %+v, want exactly one request for the whole range", fake.calls)
	}
	call := fake.calls[0]
	if call.symbol != "PETR4" || !call.from.Equal(date(2026, 9, 20)) || !call.to.Equal(date(2026, 9, 30)) {
		t.Errorf("call = %+v, want PETR4 from 2026-09-20 to 2026-09-30 (yesterday: today is the daily run's)", call)
	}

	stored := quotesOf(t, ctx, conn, asset.ID)
	if len(stored) != 11 || !stored[0].QuotedOn.Equal(date(2026, 9, 20)) || stored[0].Source != marketdata.ProviderYahoo {
		t.Errorf("stored quotes = %+v, want 11 days from 2026-09-20 sourced from brapi", stored)
	}
	if coverage := coverageOf(t, ctx, conn, asset.ID); coverage == nil ||
		!coverage.From.Equal(date(2026, 9, 20)) || !coverage.To.Equal(date(2026, 9, 30)) ||
		coverage.Source != string(marketdata.MarketB3) || coverage.Symbol != "PETR4" {
		t.Errorf("coverage = %+v, want brapi/PETR4 over 2026-09-20..2026-09-30", coverage)
	}

	// Nothing is missing any more: the next run asks for nothing.
	summary = mustRefreshHistory(t, ctx, conn, service, today)
	if summary.HistoriesFetched != 0 || len(fake.calls) != 1 {
		t.Errorf("second run: summary = %+v, calls = %d; want no new request", summary, len(fake.calls))
	}
}

func TestRefreshHistoryExtendsBackwardsWhenAnEarlierPositionAppears(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 20))
	fake := &fakeHistoryConnector{}
	service := marketdata.New(fake)
	today := date(2026, 10, 1)
	mustRefreshHistory(t, ctx, conn, service, today)

	mustOpenPosition(t, ctx, conn, "Corretora B", asset, date(2026, 9, 10))
	mustRefreshHistory(t, ctx, conn, service, today)

	if len(fake.calls) != 2 {
		t.Fatalf("calls = %+v, want a second request for the new days only", fake.calls)
	}
	if call := fake.calls[1]; !call.from.Equal(date(2026, 9, 10)) || !call.to.Equal(date(2026, 9, 19)) {
		t.Errorf("second call = %+v, want only 2026-09-10..2026-09-19", call)
	}
	if coverage := coverageOf(t, ctx, conn, asset.ID); !coverage.From.Equal(date(2026, 9, 10)) || !coverage.To.Equal(date(2026, 9, 30)) {
		t.Errorf("coverage = %+v, want the union 2026-09-10..2026-09-30", coverage)
	}
	if got := len(quotesOf(t, ctx, conn, asset.ID)); got != 21 {
		t.Errorf("stored quotes = %d, want 21 (one per day, nothing duplicated)", got)
	}
}

func TestRefreshHistoryCatchesUpDaysAMissedRunLeftOut(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 28))
	fake := &fakeHistoryConnector{}
	service := marketdata.New(fake)
	mustRefreshHistory(t, ctx, conn, service, date(2026, 10, 1))

	// The process was down on the 1st and 2nd.
	mustRefreshHistory(t, ctx, conn, service, date(2026, 10, 4))

	if len(fake.calls) != 2 {
		t.Fatalf("calls = %+v, want one catch-up request", fake.calls)
	}
	if call := fake.calls[1]; !call.from.Equal(date(2026, 10, 1)) || !call.to.Equal(date(2026, 10, 3)) {
		t.Errorf("catch-up call = %+v, want 2026-10-01..2026-10-03", call)
	}
}

func TestRefreshHistoryAsksAgainWhenTheSourceOrSymbolChanges(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 20))
	fake := &fakeHistoryConnector{}
	service := marketdata.New(fake)
	today := date(2026, 10, 1)
	mustRefreshHistory(t, ctx, conn, service, today)

	// The user corrects the symbol: what was fetched for the old one says
	// nothing about the new one.
	other := "VALE3"
	source := string(marketdata.MarketB3)
	ticker := "PETR4"
	if _, err := investments.UpdateAsset(ctx, conn, asset.ID, investments.AssetInput{
		Name: asset.Name, Ticker: &ticker, AssetType: asset.AssetType, CurrencyCode: "BRL",
		QuoteSource: &source, QuoteSymbol: &other,
	}); err != nil {
		t.Fatalf("UpdateAsset: %v", err)
	}
	mustRefreshHistory(t, ctx, conn, service, today)

	if len(fake.calls) != 2 || fake.calls[1].symbol != "VALE3" || !fake.calls[1].from.Equal(date(2026, 9, 20)) {
		t.Fatalf("calls = %+v, want the whole range asked again for VALE3", fake.calls)
	}
}

func TestRefreshHistoryUsesCanonicalCryptoTickerWithoutWritingAsset(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Bitcoin", "btc-usd", "Criptoativo", string(marketdata.MarketCrypto))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 25))
	fake := &fakeHistoryConnector{}
	mustRefreshHistory(t, ctx, conn, marketdata.New(fake), date(2026, 10, 1))
	if len(fake.calls) != 1 || fake.calls[0].symbol != "BTC" {
		t.Fatalf("calls = %+v, want BTC", fake.calls)
	}
	stored, err := investments.GetAsset(ctx, conn, asset.ID)
	if err != nil || stored.QuoteSymbol != nil {
		t.Fatalf("stored asset = %+v, err = %v", stored, err)
	}
}

func TestRefreshHistoryDerivesABrapiSymbolFromAMessyTicker(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "petr4.sa", "Ação", string(marketdata.MarketB3))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 25))
	fake := &fakeHistoryConnector{}

	mustRefreshHistory(t, ctx, conn, marketdata.New(fake), date(2026, 10, 1))

	if len(fake.calls) != 1 || fake.calls[0].symbol != "PETR4" {
		t.Fatalf("calls = %+v, want the connector asked for PETR4, not petr4.sa", fake.calls)
	}
}

// A source that asks to be left alone is not a failure: the rest of its
// assets wait for the next run, another source goes on, and nothing is marked
// as covered, so the next run asks again.
func TestRefreshHistoryDefersARateLimitedSourceWithoutFailing(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	petr := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3))
	vale := mustCreateTickerAsset(t, ctx, conn, "Vale ON", "VALE3", "Ação", string(marketdata.MarketB3))
	btc := mustCreateTickerAsset(t, ctx, conn, "Bitcoin", "BTC", "Criptoativo", string(marketdata.MarketCrypto))
	for _, asset := range []investments.Asset{petr, vale, btc} {
		mustOpenPosition(t, ctx, conn, "Conta "+asset.Name, asset, date(2026, 9, 25))
	}
	brapi := &fakeHistoryConnector{market: marketdata.MarketB3, err: &marketdata.RateLimitedError{Provider: marketdata.ProviderYahoo, RetryAfter: 2 * time.Minute}}
	gecko := &fakeHistoryConnector{market: marketdata.MarketCrypto}
	service := marketdata.New(brapi, gecko)
	today := date(2026, 10, 1)

	summary := mustRefreshHistory(t, ctx, conn, service, today)

	if summary.Failures != 0 {
		t.Errorf("Failures = %d, want 0: being asked to wait is not a failure", summary.Failures)
	}
	if summary.Deferred != 2 {
		t.Errorf("Deferred = %d, want 2 (the asset that hit the limit and the one behind it)", summary.Deferred)
	}
	if len(brapi.calls) != 2 {
		t.Errorf("brapi calls = %d, want 2: each asset reaches the service; its limiter handles the pause", len(brapi.calls))
	}
	if len(gecko.calls) != 1 || summary.HistoriesFetched != 1 {
		t.Errorf("coingecko calls = %d, fetched = %d; want the other source to go on", len(gecko.calls), summary.HistoriesFetched)
	}
	for _, asset := range []investments.Asset{petr, vale} {
		if coverage := coverageOf(t, ctx, conn, asset.ID); coverage != nil {
			t.Errorf("%s coverage = %+v, want none: nothing was obtained", asset.Name, coverage)
		}
	}

	// The limit is over: the next run takes both.
	brapi.err = nil
	summary = mustRefreshHistory(t, ctx, conn, service, today)
	if summary.HistoriesFetched != 2 || summary.Deferred != 0 {
		t.Errorf("next run summary = %+v, want both brapi assets fetched", summary)
	}
}

// What is recorded is what was asked: a source that will not serve the older
// days (CoinGecko's public API stops at 365 days back) must not be asked for
// them again on every run.
func TestRefreshHistoryRecordsWhatWasAskedWhenTheSourceServesLess(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Bitcoin", "BTC", "Criptoativo", string(marketdata.MarketCrypto))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 1))
	reach := date(2026, 9, 20)
	fake := &fakeHistoryConnector{servedFrom: &reach}
	service := marketdata.New(fake)
	today := date(2026, 10, 1)

	summary := mustRefreshHistory(t, ctx, conn, service, today)
	if summary.PricesStored != 11 {
		t.Fatalf("PricesStored = %d, want only the 11 days from 2026-09-20 that were served", summary.PricesStored)
	}
	if coverage := coverageOf(t, ctx, conn, asset.ID); !coverage.From.Equal(date(2026, 9, 1)) {
		t.Errorf("coverage from = %s, want 2026-09-01: the whole range was asked", coverage.From.Format("2006-01-02"))
	}

	mustRefreshHistory(t, ctx, conn, service, today)
	if len(fake.calls) != 1 {
		t.Errorf("calls = %d, want 1: the unreachable days are not asked for again", len(fake.calls))
	}
}

func TestRefreshHistoryFallbackKeepsUnservedDaysMissing(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 1))
	reach := date(2026, 9, 20)
	primary := &fakeHistoryConnector{err: marketdata.ErrUnavailable}
	fallback := &fakeHistoryConnector{provider: marketdata.ProviderBrapi, servedFrom: &reach}
	service := marketdata.New(primary, fallback)
	today := date(2026, 10, 1)
	if summary := mustRefreshHistory(t, ctx, conn, service, today); summary.PricesStored != 11 {
		t.Fatalf("summary = %+v, want 11 reachable days", summary)
	}
	coverage := coverageOf(t, ctx, conn, asset.ID)
	if coverage == nil || !coverage.From.Equal(reach) || coverage.Source != string(marketdata.MarketB3) {
		t.Fatalf("coverage = %+v, want b3 from 2026-09-20", coverage)
	}
	if quotes := quotesOf(t, ctx, conn, asset.ID); quotes[0].Source != marketdata.ProviderBrapi {
		t.Fatalf("first quote source = %s, want brapi", quotes[0].Source)
	}
	primary.err = nil
	if summary := mustRefreshHistory(t, ctx, conn, service, today); summary.PricesStored != 19 {
		t.Fatalf("retry summary = %+v, want 19 earlier days", summary)
	}
	coverage = coverageOf(t, ctx, conn, asset.ID)
	if !coverage.From.Equal(date(2026, 9, 1)) {
		t.Fatalf("coverage = %+v, want full range", coverage)
	}
}

func TestRefreshHistoryFallbackDoesNotBridgeGapAfterExistingCoverage(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 28))
	primary := &fakeHistoryConnector{}
	reach := date(2026, 10, 3)
	fallback := &fakeHistoryConnector{provider: marketdata.ProviderBrapi, servedFrom: &reach}
	service := marketdata.New(primary, fallback)
	mustRefreshHistory(t, ctx, conn, service, date(2026, 10, 1))
	primary.err = marketdata.ErrUnavailable
	mustRefreshHistory(t, ctx, conn, service, date(2026, 10, 5))
	coverage := coverageOf(t, ctx, conn, asset.ID)
	if !coverage.To.Equal(date(2026, 9, 30)) {
		t.Fatalf("coverage = %+v, want end 2026-09-30 so October 1-2 are retried", coverage)
	}
	primary.err = nil
	mustRefreshHistory(t, ctx, conn, service, date(2026, 10, 5))
	if coverage = coverageOf(t, ctx, conn, asset.ID); !coverage.To.Equal(date(2026, 10, 4)) {
		t.Fatalf("coverage = %+v, want gap repaired through 2026-10-04", coverage)
	}
}

func TestRefreshHistoryFallbackEntirelyTooShortLeavesCoverageEmpty(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 1))
	reach := date(2026, 10, 1)
	service := marketdata.New(&fakeHistoryConnector{err: marketdata.ErrUnavailable}, &fakeHistoryConnector{provider: marketdata.ProviderBrapi, servedFrom: &reach})
	if summary := mustRefreshHistory(t, ctx, conn, service, date(2026, 10, 1)); summary.HistoriesFetched != 1 || summary.PricesStored != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if coverage := coverageOf(t, ctx, conn, asset.ID); coverage != nil {
		t.Fatalf("coverage = %+v, want none", coverage)
	}
}

func TestRefreshHistoryLeavesAPositionOpenedTodayToTheDailyRun(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3))
	mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 10, 1))
	fake := &fakeHistoryConnector{}

	summary := mustRefreshHistory(t, ctx, conn, marketdata.New(fake), date(2026, 10, 1))

	if len(fake.calls) != 0 || summary.AssetsConsidered != 0 {
		t.Errorf("calls = %+v, summary = %+v; want nothing to backfill for a position opened today", fake.calls, summary)
	}
}

func TestRefreshHistoryIgnoresAssetsWithoutAQuoteSourceOrAHolding(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	ticker := "ITUB4"
	unquoted, err := investments.CreateAsset(ctx, conn, investments.AssetInput{
		Name: "Itaú PN", Ticker: &ticker, AssetType: "Ação", CurrencyCode: "BRL",
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	mustOpenPosition(t, ctx, conn, "Corretora A", unquoted, date(2026, 9, 1))
	mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3)) // quoted, but nobody holds it
	fake := &fakeHistoryConnector{}

	summary := mustRefreshHistory(t, ctx, conn, marketdata.New(fake), date(2026, 10, 1))

	if len(fake.calls) != 0 || summary.AssetsConsidered != 0 {
		t.Errorf("calls = %+v, summary = %+v; want nothing asked", fake.calls, summary)
	}
}

func TestRefreshAllResolvesTheSymbolFromTheTickerToo(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "bvmf:petr4", "Ação", string(marketdata.MarketB3))
	position := mustOpenPosition(t, ctx, conn, "Corretora A", asset, date(2026, 9, 1))
	fake := &fakeHistoryConnector{spot: map[string]decimal.Decimal{"PETR4": decimal.NewFromInt(50)}}

	summary, err := RefreshAll(ctx, conn, marketdata.New(fake), date(2026, 10, 1))
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.PricesFetched != 1 {
		t.Fatalf("summary = %+v, want the spot price for PETR4 fetched", summary)
	}
	if got, err := investments.GetPosition(ctx, conn, position.ID); err != nil || got.CurrentValue.String() != "500" {
		t.Errorf("position = %+v, %v; want 10 units at 50", got, err)
	}
}

func TestRefreshAllDefersARateLimitedSourceInsteadOfFailingEveryAsset(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	for _, ticker := range []string{"PETR4", "VALE3", "ITUB4"} {
		asset := mustCreateTickerAsset(t, ctx, conn, "Ação "+ticker, ticker, "Ação", string(marketdata.MarketB3))
		mustOpenPosition(t, ctx, conn, "Conta "+ticker, asset, date(2026, 9, 1))
	}
	fake := &fakeHistoryConnector{err: &marketdata.RateLimitedError{Provider: marketdata.ProviderYahoo, RetryAfter: time.Minute}}

	summary, err := RefreshAll(ctx, conn, marketdata.New(fake), date(2026, 10, 1))
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.PriceFailures != 0 || summary.PricesDeferred != 3 {
		t.Errorf("summary = %+v, want 3 deferred and no failures", summary)
	}
}

// A save wakes the scheduler between daily runs: an asset that was just
// quoted gets today's price right away, and one the daily run already priced
// costs no request.
func TestRefreshMissingPricesOnlyWhatHasNoPriceToday(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	priced := mustCreateTickerAsset(t, ctx, conn, "Petrobras PN", "PETR4", "Ação", string(marketdata.MarketB3))
	fresh := mustCreateTickerAsset(t, ctx, conn, "Vale ON", "VALE3", "Ação", string(marketdata.MarketB3))
	mustOpenPosition(t, ctx, conn, "Conta PETR4", priced, date(2026, 9, 1))
	mustOpenPosition(t, ctx, conn, "Conta VALE3", fresh, date(2026, 9, 1))
	fake := &fakeHistoryConnector{spot: map[string]decimal.Decimal{"PETR4": decimal.NewFromInt(50), "VALE3": decimal.NewFromInt(60)}}
	service := marketdata.New(fake)
	today := date(2026, 10, 1)

	// The daily run priced PETR4 this morning.
	if err := investments.UpsertAssetQuote(ctx, conn, investments.AssetQuote{
		AssetID: priced.ID, QuotedOn: today, Price: decimal.NewFromInt(49),
		Source: marketdata.ProviderYahoo, Origin: investments.QuoteOriginMarket,
	}); err != nil {
		t.Fatalf("UpsertAssetQuote: %v", err)
	}

	summary, err := RefreshMissing(ctx, conn, service, today)
	if err != nil {
		t.Fatalf("RefreshMissing: %v", err)
	}
	if summary.PricesFetched != 1 || summary.AssetsConsidered != 1 {
		t.Fatalf("summary = %+v, want only VALE3 priced", summary)
	}
	if got := quotesOf(t, ctx, conn, fresh.ID); len(got) != 1 || got[0].Price.String() != "60" {
		t.Errorf("VALE3 quotes = %+v, want today's 60", got)
	}
	if got := quotesOf(t, ctx, conn, priced.ID); len(got) != 1 || got[0].Price.String() != "49" {
		t.Errorf("PETR4 quotes = %+v, want the morning's 49 untouched", got)
	}
}
