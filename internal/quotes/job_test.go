package quotes

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/investments"
	"contadinho-go/internal/marketdata"
)

// fakeConnector is an in-memory Connector: no HTTP at all, so job tests
// exercise only RefreshAll's orchestration, never a real network call.
type fakeConnector struct {
	prices map[string]decimal.Decimal
	err    error
	asked  []string // symbols quoted, in order
}

func (f *fakeConnector) Name() string { return marketdata.ProviderYahoo }
func (f *fakeConnector) Supports(market marketdata.Market) bool {
	return market == marketdata.MarketCrypto
}
func (f *fakeConnector) Quote(_ context.Context, instrument marketdata.Instrument) (marketdata.Quote, error) {
	symbol := instrument.Symbol
	f.asked = append(f.asked, symbol)
	if f.err != nil {
		return marketdata.Quote{}, f.err
	}
	price, ok := f.prices[symbol]
	if !ok {
		return marketdata.Quote{}, fmt.Errorf("fake connector: no price for %q", symbol)
	}
	return marketdata.Quote{Price: price, Currency: "BRL"}, nil
}

func (f *fakeConnector) History(context.Context, marketdata.Instrument, time.Time, time.Time) (marketdata.History, error) {
	return marketdata.History{}, fmt.Errorf("not used")
}

// newJobTestConn mirrors internal/investments' bookFixture: a real migrated
// sqlite database in t.TempDir(), through db.Open (which applies every
// embedded migration, including this feature's ADD COLUMNs).
func newJobTestConn(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func mustCreateQuotedAsset(t *testing.T, ctx context.Context, conn *sql.DB, name, assetType, source, symbol string) investments.Asset {
	t.Helper()
	asset, err := investments.CreateAsset(ctx, conn, investments.AssetInput{
		Name: name, AssetType: assetType, CurrencyCode: "BRL",
		QuoteSource: &source, QuoteSymbol: &symbol,
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	return asset
}

// mustCreateManualPosition opens a fresh manual account and a position of
// quantity units of asset in it, dated well before any "today" used in
// these tests.
func mustCreateManualPosition(t *testing.T, ctx context.Context, conn *sql.DB, accountName string, asset investments.Asset, quantity int64) investments.Position {
	t.Helper()
	account, err := investments.CreateAccount(ctx, conn, investments.AccountInput{Name: accountName})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	position, err := investments.CreatePosition(ctx, conn, investments.PositionInput{
		AccountID: account.ID, AssetID: &asset.ID, Name: asset.Name, AssetType: asset.AssetType,
		InitialQuantity: decimal.NewFromInt(quantity), InitialUnitCost: decimal.NewFromInt(1),
		OccurredOn: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreatePosition: %v", err)
	}
	return position
}

func listPositionOperations(t *testing.T, ctx context.Context, conn *sql.DB, positionID string) []investments.Operation {
	t.Helper()
	ops, err := investments.ListOperations(ctx, conn, investments.OperationFilter{PositionID: &positionID})
	if err != nil {
		t.Fatalf("ListOperations: %v", err)
	}
	return ops
}

// onlyOpeningBalance fails the test unless the position's ledger is just the
// initial_balance operation mustCreateManualPosition writes: the job records
// prices in the series, never operations.
func onlyOpeningBalance(t *testing.T, ctx context.Context, conn *sql.DB, positionID string) investments.Operation {
	t.Helper()
	ops := listPositionOperations(t, ctx, conn, positionID)
	if len(ops) != 1 || ops[0].Kind != investments.OperationInitialBalance {
		t.Fatalf("operations = %+v, want only the opening balance", ops)
	}
	return ops[0]
}

func currentPosition(t *testing.T, ctx context.Context, conn *sql.DB, positionID string) investments.Position {
	t.Helper()
	position, err := investments.GetPosition(ctx, conn, positionID)
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	return position
}

func TestRefreshAllStoresTheDaysPriceInTheSeries(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", string(marketdata.MarketCrypto), "BTC")
	position := mustCreateManualPosition(t, ctx, conn, "Corretora A", asset, 2)

	service := marketdata.New(&fakeConnector{prices: map[string]decimal.Decimal{"BTC": decimal.NewFromInt(100000)}})
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	summary, err := RefreshAll(ctx, conn, service, today)
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.AssetsConsidered != 1 || summary.PricesFetched != 1 || summary.PriceFailures != 0 {
		t.Fatalf("summary = %+v, want 1 asset priced", summary)
	}

	stored := quotesOf(t, ctx, conn, asset.ID)
	if len(stored) != 1 || !stored[0].QuotedOn.Equal(today) || stored[0].Price.String() != "100000" || stored[0].Source != marketdata.ProviderYahoo {
		t.Fatalf("series = %+v, want today's 100000 from the provider that served it", stored)
	}
	onlyOpeningBalance(t, ctx, conn, position.ID)

	got := currentPosition(t, ctx, conn, position.ID)
	if got.ValuationBasis != investments.ValuationBasisMarketQuote || got.CurrentValue.String() != "200000" {
		t.Errorf("position = basis %s value %s, want market_quote and 2 × 100000", got.ValuationBasis, got.CurrentValue)
	}
	if got.ValuedOn == nil || !got.ValuedOn.Equal(today) {
		t.Errorf("valued on = %v, want %s", got.ValuedOn, today.Format(investments.DateLayout))
	}
}

func TestRefreshAllUpdatesTheSeriesRowOnASecondRunSameDay(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", string(marketdata.MarketCrypto), "BTC")
	position := mustCreateManualPosition(t, ctx, conn, "Corretora A", asset, 2)
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	service := marketdata.New(&fakeConnector{prices: map[string]decimal.Decimal{"BTC": decimal.NewFromInt(100000)}})
	if _, err := RefreshAll(ctx, conn, service, today); err != nil {
		t.Fatalf("first RefreshAll: %v", err)
	}

	service = marketdata.New(&fakeConnector{prices: map[string]decimal.Decimal{"BTC": decimal.NewFromInt(150000)}})
	summary, err := RefreshAll(ctx, conn, service, today)
	if err != nil {
		t.Fatalf("second RefreshAll: %v", err)
	}
	if summary.PricesFetched != 1 {
		t.Fatalf("summary = %+v, want the second run to price the asset again", summary)
	}

	stored := quotesOf(t, ctx, conn, asset.ID)
	if len(stored) != 1 || stored[0].Price.String() != "150000" {
		t.Fatalf("series = %+v, want the same day's row updated to 150000, not a second row", stored)
	}
	onlyOpeningBalance(t, ctx, conn, position.ID)
	if got := currentPosition(t, ctx, conn, position.ID); got.CurrentValue.String() != "300000" {
		t.Errorf("value = %s, want 2 × 150000", got.CurrentValue)
	}
}

// A valuation a person typed today is as recent as today's price, and on a tie
// theirs wins: the job neither touches the operation nor displaces its value.
func TestRefreshAllLeavesATypedValuationOfTodayWinning(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", string(marketdata.MarketCrypto), "BTC")
	position := mustCreateManualPosition(t, ctx, conn, "Corretora A", asset, 2)
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	typed, err := investments.CreateOperation(ctx, conn, investments.OperationInput{
		AccountID: position.AccountID, PositionID: &position.ID, Kind: investments.OperationValuation,
		OccurredOn: today, Amount: decimal.NewFromInt(999999),
	})
	if err != nil {
		t.Fatalf("typed valuation: %v", err)
	}

	service := marketdata.New(&fakeConnector{prices: map[string]decimal.Decimal{"BTC": decimal.NewFromInt(100000)}})
	summary, err := RefreshAll(ctx, conn, service, today)
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.PricesFetched != 1 {
		t.Fatalf("summary = %+v, want the price stored all the same", summary)
	}

	ops := listPositionOperations(t, ctx, conn, position.ID)
	if len(ops) != 2 {
		t.Fatalf("operations = %+v, want the opening balance and the typed valuation", ops)
	}
	for _, op := range ops {
		if op.Kind != investments.OperationValuation {
			continue
		}
		if op.ID != typed.ID || op.Amount.String() != "999999" || op.Notes != nil || !op.UpdatedAt.Equal(typed.UpdatedAt) {
			t.Errorf("valuation = %+v, want the typed operation untouched", op)
		}
	}
	got := currentPosition(t, ctx, conn, position.ID)
	if got.ValuationBasis != investments.ValuationBasisManualValuation || got.CurrentValue.String() != "999999" {
		t.Errorf("position = basis %s value %s, want the typed 999999", got.ValuationBasis, got.CurrentValue)
	}
	if stored := quotesOf(t, ctx, conn, asset.ID); len(stored) != 1 || stored[0].Price.String() != "100000" {
		t.Errorf("series = %+v, want today's price recorded", stored)
	}
}

// Only an asset some manual position holds units of is worth a request: not
// one nobody holds, one whose position is still empty, nor one sold out.
func TestRefreshAllSkipsAssetsNobodyHoldsUnitsOf(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	prices := map[string]decimal.Decimal{"BTC": decimal.NewFromInt(100000), "ETH": decimal.NewFromInt(5000), "SOL": decimal.NewFromInt(900), "ADA": decimal.NewFromInt(3)}
	unheld := mustCreateQuotedAsset(t, ctx, conn, "Cardano", "Criptoativo", string(marketdata.MarketCrypto), "ADA")
	emptyAsset := mustCreateQuotedAsset(t, ctx, conn, "Ether", "Criptoativo", string(marketdata.MarketCrypto), "ETH")
	soldOutAsset := mustCreateQuotedAsset(t, ctx, conn, "Solana", "Criptoativo", string(marketdata.MarketCrypto), "SOL")
	heldAsset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", string(marketdata.MarketCrypto), "BTC")

	account, err := investments.CreateAccount(ctx, conn, investments.AccountInput{Name: "Corretora A"})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	// No initial quantity/cost/value: opens with zero units and no operation.
	// Such a position replays with no ledger state at all, so it is not Closed
	// even though Quantity is zero: the job reads the quantity itself.
	empty, err := investments.CreatePosition(ctx, conn, investments.PositionInput{
		AccountID: account.ID, AssetID: &emptyAsset.ID, Name: emptyAsset.Name, AssetType: emptyAsset.AssetType,
	})
	if err != nil {
		t.Fatalf("CreatePosition (empty): %v", err)
	}
	if !empty.Quantity.IsZero() {
		t.Fatalf("expected a zero-quantity position, got %+v", empty)
	}
	soldOut := mustCreateManualPosition(t, ctx, conn, "Corretora B", soldOutAsset, 2)
	if _, err := investments.CreateOperation(ctx, conn, investments.OperationInput{
		AccountID: soldOut.AccountID, PositionID: &soldOut.ID, Kind: investments.OperationSell,
		OccurredOn: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Amount: decimal.NewFromInt(2), Quantity: decimalPtr(2),
	}); err != nil {
		t.Fatalf("sell: %v", err)
	}
	mustCreateManualPosition(t, ctx, conn, "Corretora C", heldAsset, 1)

	fake := &fakeConnector{prices: prices}
	summary, err := RefreshAll(ctx, conn, marketdata.New(fake), time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.AssetsConsidered != 1 || summary.PricesFetched != 1 {
		t.Fatalf("summary = %+v, want only the held asset priced", summary)
	}
	if len(fake.asked) != 1 || fake.asked[0] != "BTC" {
		t.Fatalf("connector asked for %v, want only BTC", fake.asked)
	}
	for _, skipped := range []investments.Asset{unheld, emptyAsset, soldOutAsset} {
		if stored := quotesOf(t, ctx, conn, skipped.ID); len(stored) != 0 {
			t.Errorf("%s series = %+v, want no price", skipped.Name, stored)
		}
	}
}

func decimalPtr(v int64) *decimal.Decimal {
	d := decimal.NewFromInt(v)
	return &d
}

func TestRefreshAllOneAssetFailureDoesNotBlockAnother(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	broken := mustCreateQuotedAsset(t, ctx, conn, "Moeda Quebrada", "Criptoativo", string(marketdata.MarketCrypto), "BROKEN")
	brokenPosition := mustCreateManualPosition(t, ctx, conn, "Corretora A", broken, 1)
	ok := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", string(marketdata.MarketCrypto), "BTC")
	okPosition := mustCreateManualPosition(t, ctx, conn, "Corretora B", ok, 1)

	service := marketdata.New(&fakeConnector{
		// No entry for "BROKEN": the fake connector errors on it.
		prices: map[string]decimal.Decimal{"BTC": decimal.NewFromInt(100000)},
	})
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	summary, err := RefreshAll(ctx, conn, service, today)
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.PriceFailures != 1 || summary.PricesFetched != 1 {
		t.Fatalf("summary = %+v, want 1 failure and 1 priced", summary)
	}

	if stored := quotesOf(t, ctx, conn, broken.ID); len(stored) != 0 {
		t.Fatalf("broken asset series = %+v, want none", stored)
	}
	if got := currentPosition(t, ctx, conn, brokenPosition.ID); got.ValuationBasis != investments.ValuationBasisCostBasis {
		t.Errorf("broken asset position = %+v, want it left at cost", got)
	}
	if got := currentPosition(t, ctx, conn, okPosition.ID); got.CurrentValue.String() != "100000" {
		t.Errorf("ok asset value = %s, want 100000", got.CurrentValue)
	}
}

// TestRefreshAllPricesSameAssetHeldInDifferentAccounts mirrors
// internal/investments' TestSameAssetCanBeHeldInDifferentAccounts guarantee:
// one asset, two manual positions in two different accounts, one price in the
// series and each position valued from it.
func TestRefreshAllPricesSameAssetHeldInDifferentAccounts(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", string(marketdata.MarketCrypto), "BTC")
	first := mustCreateManualPosition(t, ctx, conn, "Corretora A", asset, 1)
	second := mustCreateManualPosition(t, ctx, conn, "Corretora B", asset, 3)

	fake := &fakeConnector{prices: map[string]decimal.Decimal{"BTC": decimal.NewFromInt(100000)}}
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	summary, err := RefreshAll(ctx, conn, marketdata.New(fake), today)
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.AssetsConsidered != 1 || summary.PricesFetched != 1 || len(fake.asked) != 1 {
		t.Fatalf("summary = %+v, asked = %v; want the asset priced once however many accounts hold it", summary, fake.asked)
	}
	if stored := quotesOf(t, ctx, conn, asset.ID); len(stored) != 1 {
		t.Fatalf("series = %+v, want a single row", stored)
	}

	for _, tc := range []struct {
		position investments.Position
		want     string
	}{{first, "100000"}, {second, "300000"}} {
		onlyOpeningBalance(t, ctx, conn, tc.position.ID)
		if got := currentPosition(t, ctx, conn, tc.position.ID); got.CurrentValue.String() != tc.want {
			t.Errorf("position %s value = %s, want %s", tc.position.ID, got.CurrentValue, tc.want)
		}
	}
}

// The market price must not replace what the provider itself reported for the
// day: a Pluggy quote outranks a spot from a market data provider.
func TestRefreshAllDoesNotOverwriteAPriceTheProviderReported(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", string(marketdata.MarketCrypto), "BTC")
	mustCreateManualPosition(t, ctx, conn, "Corretora A", asset, 1)
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	if err := investments.UpsertAssetQuote(ctx, conn, investments.AssetQuote{
		AssetID: asset.ID, QuotedOn: today, Price: decimal.NewFromInt(95000),
		Source: investments.QuoteSourcePluggy, Origin: investments.QuoteOriginSync,
	}); err != nil {
		t.Fatalf("UpsertAssetQuote: %v", err)
	}

	service := marketdata.New(&fakeConnector{prices: map[string]decimal.Decimal{"BTC": decimal.NewFromInt(100000)}})
	if _, err := RefreshAll(ctx, conn, service, today); err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if stored := quotesOf(t, ctx, conn, asset.ID); len(stored) != 1 || stored[0].Origin != investments.QuoteOriginSync || stored[0].Price.String() != "95000" {
		t.Fatalf("series = %+v, want the Pluggy price kept", stored)
	}
}

// Only a market price makes an asset priced for the day: what the sync
// observed or an issue PU says nothing about the market price, so RefreshMissing
// still asks for it. It cannot replace what the sync observed, though.
func TestPricedTodayCountsOnlyMarketPrices(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name, symbol, source string
		origin               investments.QuoteOrigin
		want                 bool
	}{
		{"market price", "MKT", marketdata.ProviderBrapi, investments.QuoteOriginMarket, true},
		{"sync price", "SYN", investments.QuoteSourcePluggy, investments.QuoteOriginSync, false},
		{"issue price", "ISS", investments.QuoteSourceIssue, investments.QuoteOriginIssue, false},
	} {
		asset := mustCreateQuotedAsset(t, ctx, conn, tc.name, "Criptoativo", string(marketdata.MarketCrypto), tc.symbol)
		if err := investments.UpsertAssetQuote(ctx, conn, investments.AssetQuote{
			AssetID: asset.ID, QuotedOn: today, Price: decimal.NewFromInt(10), Source: tc.source, Origin: tc.origin,
		}); err != nil {
			t.Fatalf("%s: UpsertAssetQuote: %v", tc.name, err)
		}
		got, err := pricedToday(ctx, conn, asset.ID, today)
		if err != nil || got != tc.want {
			t.Errorf("%s: pricedToday = %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}

	// An older market price does not make today priced.
	asset := mustCreateQuotedAsset(t, ctx, conn, "Ontem", "Criptoativo", string(marketdata.MarketCrypto), "OLD")
	if err := investments.UpsertAssetQuote(ctx, conn, investments.AssetQuote{
		AssetID: asset.ID, QuotedOn: today.AddDate(0, 0, -1), Price: decimal.NewFromInt(10),
		Source: marketdata.ProviderYahoo, Origin: investments.QuoteOriginMarket,
	}); err != nil {
		t.Fatalf("UpsertAssetQuote: %v", err)
	}
	if got, err := pricedToday(ctx, conn, asset.ID, today); err != nil || got {
		t.Errorf("yesterday's price: pricedToday = %v, %v; want false", got, err)
	}
}

func TestRefreshMissingStillAsksWhenOnlyTheSyncPricedTheDay(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", string(marketdata.MarketCrypto), "BTC")
	mustCreateManualPosition(t, ctx, conn, "Corretora A", asset, 1)
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	if err := investments.UpsertAssetQuote(ctx, conn, investments.AssetQuote{
		AssetID: asset.ID, QuotedOn: today, Price: decimal.NewFromInt(95000),
		Source: investments.QuoteSourcePluggy, Origin: investments.QuoteOriginSync,
	}); err != nil {
		t.Fatalf("UpsertAssetQuote: %v", err)
	}

	fake := &fakeConnector{prices: map[string]decimal.Decimal{"BTC": decimal.NewFromInt(100000)}}
	summary, err := RefreshMissing(ctx, conn, marketdata.New(fake), today)
	if err != nil {
		t.Fatalf("RefreshMissing: %v", err)
	}
	if summary.AssetsConsidered != 1 || len(fake.asked) != 1 {
		t.Fatalf("summary = %+v, asked = %v; want the market price asked for", summary, fake.asked)
	}
	if stored := quotesOf(t, ctx, conn, asset.ID); len(stored) != 1 || stored[0].Origin != investments.QuoteOriginSync || stored[0].Price.String() != "95000" {
		t.Fatalf("series = %+v, want the sync's price kept", stored)
	}

	// Once the market has priced another asset's day, it costs no request.
	other := mustCreateQuotedAsset(t, ctx, conn, "Ether", "Criptoativo", string(marketdata.MarketCrypto), "ETH")
	mustCreateManualPosition(t, ctx, conn, "Corretora B", other, 1)
	if err := investments.UpsertConnectorQuotes(ctx, conn, []investments.AssetQuote{{
		AssetID: other.ID, QuotedOn: today, Price: decimal.NewFromInt(5000),
		Source: marketdata.ProviderYahoo, Origin: investments.QuoteOriginMarket,
	}}); err != nil {
		t.Fatalf("UpsertConnectorQuotes: %v", err)
	}
	fake.asked = nil
	if _, err := RefreshMissing(ctx, conn, marketdata.New(fake), today); err != nil {
		t.Fatalf("RefreshMissing: %v", err)
	}
	for _, symbol := range fake.asked {
		if symbol == "ETH" {
			t.Errorf("asked for ETH, already market-priced today")
		}
	}
}
