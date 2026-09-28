package quotes

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/investments"
)

// fakeConnector is an in-memory Connector: no HTTP at all, so job tests
// exercise only RefreshAll's orchestration and dedupe, never a real
// network call.
type fakeConnector struct {
	prices map[string]decimal.Decimal
	err    error
}

func (f *fakeConnector) FetchPrice(_ context.Context, symbol string) (decimal.Decimal, error) {
	if f.err != nil {
		return decimal.Decimal{}, f.err
	}
	price, ok := f.prices[symbol]
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("fake connector: no price for %q", symbol)
	}
	return price, nil
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
// these tests so the opening operation never lands inside a same-day
// dedupe window.
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

// onlyValuation picks out the single OperationValuation among ops (ignoring
// e.g. the initial_balance operation mustCreateManualPosition's opening
// balance always creates), failing the test if there is more than one.
func onlyValuation(t *testing.T, ops []investments.Operation) *investments.Operation {
	t.Helper()
	var found *investments.Operation
	for i := range ops {
		if ops[i].Kind != investments.OperationValuation {
			continue
		}
		if found != nil {
			t.Fatalf("more than one valuation operation: %+v", ops)
		}
		found = &ops[i]
	}
	return found
}

func TestRefreshAllInsertsOnFirstRun(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", SourceCoinGecko, "bitcoin")
	position := mustCreateManualPosition(t, ctx, conn, "Corretora A", asset, 2)

	registry := Registry{SourceCoinGecko: &fakeConnector{prices: map[string]decimal.Decimal{"bitcoin": decimal.NewFromInt(100000)}}}
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	summary, err := RefreshAll(ctx, conn, registry, today)
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.PositionsCreated != 1 || summary.PositionsUpdated != 0 || summary.PositionsSkipped != 0 {
		t.Fatalf("summary = %+v, want 1 created", summary)
	}

	ops := listPositionOperations(t, ctx, conn, position.ID)
	var valuation *investments.Operation
	for i := range ops {
		if ops[i].Kind == investments.OperationValuation {
			valuation = &ops[i]
		}
	}
	if valuation == nil {
		t.Fatal("no valuation operation created")
	}
	if valuation.Amount.String() != "200000" {
		t.Errorf("amount = %s, want 200000 (2 * 100000)", valuation.Amount.String())
	}
	if valuation.Notes == nil || !strings.HasPrefix(*valuation.Notes, AutoQuoteMarker) {
		t.Errorf("notes = %v, want a value prefixed with %q", valuation.Notes, AutoQuoteMarker)
	}
	if valuation.Source != "manual" || !valuation.IsEditable {
		t.Errorf("valuation = %+v, want source=manual and editable", valuation)
	}
}

func TestRefreshAllUpdatesInPlaceOnSecondRunSameDay(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", SourceCoinGecko, "bitcoin")
	position := mustCreateManualPosition(t, ctx, conn, "Corretora A", asset, 2)
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	registry := Registry{SourceCoinGecko: &fakeConnector{prices: map[string]decimal.Decimal{"bitcoin": decimal.NewFromInt(100000)}}}
	if _, err := RefreshAll(ctx, conn, registry, today); err != nil {
		t.Fatalf("first RefreshAll: %v", err)
	}
	first := onlyValuation(t, listPositionOperations(t, ctx, conn, position.ID))
	if first == nil {
		t.Fatal("after first run: no valuation operation")
	}
	firstID, firstCreatedAt := first.ID, first.CreatedAt

	registry = Registry{SourceCoinGecko: &fakeConnector{prices: map[string]decimal.Decimal{"bitcoin": decimal.NewFromInt(150000)}}}
	summary, err := RefreshAll(ctx, conn, registry, today)
	if err != nil {
		t.Fatalf("second RefreshAll: %v", err)
	}
	if summary.PositionsUpdated != 1 || summary.PositionsCreated != 0 {
		t.Fatalf("summary = %+v, want 1 updated, 0 created", summary)
	}

	allOps := listPositionOperations(t, ctx, conn, position.ID)
	if len(allOps) != 2 {
		t.Fatalf("after second run: %d operations (%+v), want 2 (opening balance + one valuation, update in place rather than a duplicate)", len(allOps), allOps)
	}
	second := onlyValuation(t, allOps)
	if second == nil {
		t.Fatal("after second run: no valuation operation")
	}
	if second.ID != firstID {
		t.Errorf("operation id changed: %s -> %s, want the same id", firstID, second.ID)
	}
	if !second.CreatedAt.Equal(firstCreatedAt) {
		t.Errorf("created_at changed: %v -> %v, want unchanged", firstCreatedAt, second.CreatedAt)
	}
	if second.Amount.String() != "300000" {
		t.Errorf("amount = %s, want 300000 (2 * 150000)", second.Amount.String())
	}
}

func TestRefreshAllSkipsWhenHumanValuationExistsToday(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", SourceCoinGecko, "bitcoin")
	position := mustCreateManualPosition(t, ctx, conn, "Corretora A", asset, 2)
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	if _, err := investments.CreateOperation(ctx, conn, investments.OperationInput{
		AccountID: position.AccountID, PositionID: &position.ID, Kind: investments.OperationValuation,
		OccurredOn: today, Amount: decimal.NewFromInt(999999),
	}); err != nil {
		t.Fatalf("human valuation: %v", err)
	}

	registry := Registry{SourceCoinGecko: &fakeConnector{prices: map[string]decimal.Decimal{"bitcoin": decimal.NewFromInt(100000)}}}
	summary, err := RefreshAll(ctx, conn, registry, today)
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.PositionsSkipped != 1 || summary.PositionsCreated != 0 || summary.PositionsUpdated != 0 {
		t.Fatalf("summary = %+v, want 1 skipped", summary)
	}

	human := onlyValuation(t, listPositionOperations(t, ctx, conn, position.ID))
	if human == nil {
		t.Fatal("the human valuation operation disappeared")
	}
	if human.Amount.String() != "999999" {
		t.Errorf("amount = %s, want the human-entered 999999 to survive untouched", human.Amount.String())
	}
	if human.Notes != nil {
		t.Errorf("notes = %v, want nil (never stamped by the job)", human.Notes)
	}
}

func TestRefreshAllSkipsClosedPosition(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", SourceCoinGecko, "bitcoin")
	account, err := investments.CreateAccount(ctx, conn, investments.AccountInput{Name: "Corretora A"})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	// No initial quantity/cost/value: opens with zero units, i.e. already closed.
	position, err := investments.CreatePosition(ctx, conn, investments.PositionInput{
		AccountID: account.ID, AssetID: &asset.ID, Name: asset.Name, AssetType: asset.AssetType,
	})
	if err != nil {
		t.Fatalf("CreatePosition (empty): %v", err)
	}
	// A position that never had any operation replays with no ledger state
	// for it at all, so applyLedgerState never runs and Closed stays the
	// zero value (false) even though Quantity is zero too — this is
	// exactly the gap job.go's explicit Quantity.IsPositive() guard covers,
	// so this test intentionally does not assert Closed here.
	if !position.Quantity.IsZero() {
		t.Fatalf("expected a zero-quantity position, got %+v", position)
	}

	registry := Registry{SourceCoinGecko: &fakeConnector{prices: map[string]decimal.Decimal{"bitcoin": decimal.NewFromInt(100000)}}}
	summary, err := RefreshAll(ctx, conn, registry, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.PositionsCreated != 0 || summary.PositionsUpdated != 0 || summary.PositionsSkipped != 0 {
		t.Fatalf("summary = %+v, want no positions touched", summary)
	}
	if ops := listPositionOperations(t, ctx, conn, position.ID); len(ops) != 0 {
		t.Fatalf("operations for a closed position = %d, want 0", len(ops))
	}
}

func TestRefreshAllOneAssetFailureDoesNotBlockAnother(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	broken := mustCreateQuotedAsset(t, ctx, conn, "Moeda Quebrada", "Criptoativo", SourceCoinGecko, "broken-coin")
	brokenPosition := mustCreateManualPosition(t, ctx, conn, "Corretora A", broken, 1)
	ok := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", SourceCoinGecko, "bitcoin")
	okPosition := mustCreateManualPosition(t, ctx, conn, "Corretora B", ok, 1)

	registry := Registry{SourceCoinGecko: &fakeConnector{
		// No entry for "broken-coin": the fake connector errors on it.
		prices: map[string]decimal.Decimal{"bitcoin": decimal.NewFromInt(100000)},
	}}
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	summary, err := RefreshAll(ctx, conn, registry, today)
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.PriceFailures != 1 || summary.PositionsCreated != 1 {
		t.Fatalf("summary = %+v, want 1 failure and 1 created", summary)
	}

	if v := onlyValuation(t, listPositionOperations(t, ctx, conn, brokenPosition.ID)); v != nil {
		t.Fatalf("broken asset valuation = %+v, want none", v)
	}
	if v := onlyValuation(t, listPositionOperations(t, ctx, conn, okPosition.ID)); v == nil {
		t.Fatal("ok asset valuation missing")
	}
}

// TestRefreshAllPricesSameAssetHeldInDifferentAccounts mirrors
// internal/investments' TestSameAssetCanBeHeldInDifferentAccounts guarantee:
// one asset, two manual positions in two different accounts, both priced.
func TestRefreshAllPricesSameAssetHeldInDifferentAccounts(t *testing.T) {
	ctx := context.Background()
	conn := newJobTestConn(t)
	asset := mustCreateQuotedAsset(t, ctx, conn, "Bitcoin", "Criptoativo", SourceCoinGecko, "bitcoin")
	first := mustCreateManualPosition(t, ctx, conn, "Corretora A", asset, 1)
	second := mustCreateManualPosition(t, ctx, conn, "Corretora B", asset, 3)

	registry := Registry{SourceCoinGecko: &fakeConnector{prices: map[string]decimal.Decimal{"bitcoin": decimal.NewFromInt(100000)}}}
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	summary, err := RefreshAll(ctx, conn, registry, today)
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if summary.PositionsCreated != 2 {
		t.Fatalf("summary = %+v, want 2 created", summary)
	}

	for _, tc := range []struct {
		position investments.Position
		want     string
	}{{first, "100000"}, {second, "300000"}} {
		v := onlyValuation(t, listPositionOperations(t, ctx, conn, tc.position.ID))
		if v == nil {
			t.Fatalf("position %s: no valuation operation", tc.position.ID)
		}
		if v.Amount.String() != tc.want {
			t.Errorf("position %s amount = %s, want %s", tc.position.ID, v.Amount.String(), tc.want)
		}
	}
}
