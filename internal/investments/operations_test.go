package investments_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/investments"
)

// ledgerFixture is a migrated database with one bank account and one manual
// custody account, the minimum an operation or a reconciliation needs.
type ledgerFixture struct {
	t           *testing.T
	conn        *sql.DB
	ctx         context.Context
	sourceID    string
	rawImportID string
	bankID      string
	custodyID   string
}

func newLedgerFixture(t *testing.T) *ledgerFixture {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	f := &ledgerFixture{t: t, conn: conn, ctx: context.Background(), sourceID: uuid.NewString(), rawImportID: uuid.NewString()}
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES (?, 'pluggy', 'item-1', ?, ?)`, f.sourceID, now, now)
	syncRunID := uuid.NewString()
	f.exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES (?, ?, 'completed', ?, ?)`, syncRunID, f.sourceID, now, now)
	f.exec(`INSERT INTO raw_imports (
			id, sync_run_id, source_id, scope, page_sequence, request_attempt,
			request_method, request_path, http_status, response_headers, payload,
			payload_sha256, received_at
		) VALUES (?, ?, ?, 'accounts', 1, 1, 'GET', '/x', 200, '{}', x'00', 'sha', ?)`,
		f.rawImportID, syncRunID, f.sourceID, now)
	f.bankID = uuid.NewString()
	f.exec(`INSERT INTO financial_accounts (
			id, source_id, external_id, balance, currency_code,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, '5000', 'BRL', ?, 'hash', ?, ?)`,
		f.bankID, f.sourceID, f.bankID, f.rawImportID, now, now)

	account, err := investments.CreateAccount(f.ctx, conn, investments.AccountInput{Name: "Corretora"})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	f.custodyID = account.ID
	return f
}

func (f *ledgerFixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.conn.Exec(query, args...); err != nil {
		f.t.Fatalf("exec %q: %v", query, err)
	}
}

func (f *ledgerFixture) addPosition(accountID, name string) string {
	f.t.Helper()
	id, assetID := uuid.NewString(), uuid.NewString()
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO investment_assets (id, canonical_key, name, asset_type, created_at, updated_at)
		VALUES (?, ?, ?, 'EQUITY', ?, ?)`, assetID, "name:EQUITY:"+name, name, now, now)
	f.exec(`INSERT INTO investment_positions (id, account_id, asset_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`, id, accountID, assetID, now, now)
	return id
}

func (f *ledgerFixture) addBankTransaction(amount string, occurredAt time.Time) string {
	f.t.Helper()
	id := uuid.NewString()
	now := db.FormatTime(time.Now())
	movementType := "CREDIT"
	if len(amount) > 0 && amount[0] == '-' {
		movementType = "DEBIT"
	}
	f.exec(`INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description, amount, amount_in_account_currency,
			currency_code, occurred_at, provider_status, movement_type, current_raw_import_id,
			normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'Movimento', ?, ?, 'BRL', ?, 'POSTED', ?, ?, 'hash', ?, ?)`,
		id, f.sourceID, f.bankID, id, amount, amount, db.FormatTime(occurredAt), movementType, f.rawImportID, now, now)
	return id
}

func (f *ledgerFixture) addInvestmentMovement() string {
	f.t.Helper()
	investmentID := uuid.NewString()
	id := uuid.NewString()
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO financial_investments (
			id, source_id, external_id, balance, currency_code,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, '1000', 'BRL', ?, 'hash', ?, ?)`,
		investmentID, f.sourceID, investmentID, f.rawImportID, now, now)
	f.exec(`INSERT INTO financial_investment_transactions (
			id, source_id, investment_id, external_id, movement_type, amount, direction, occurred_at,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'APPLICATION', '1000', 'inflow', ?, ?, 'hash', ?, ?)`,
		id, f.sourceID, investmentID, id, db.FormatTime(day(1)), f.rawImportID, now, now)
	return id
}

func (f *ledgerFixture) cash() string {
	f.t.Helper()
	account, err := investments.GetAccount(f.ctx, f.conn, f.custodyID)
	if err != nil {
		f.t.Fatalf("GetAccount: %v", err)
	}
	return account.CashBalance.String()
}

func (f *ledgerFixture) assertCash(label, want string) {
	f.t.Helper()
	if got := f.cash(); got != want {
		f.t.Fatalf("%s cash = %s, want %s", label, got, want)
	}
}

func day(n int) time.Time { return time.Date(2026, 9, n, 0, 0, 0, 0, time.UTC) }

func dec(v string) decimal.Decimal {
	d, err := decimal.NewFromString(v)
	if err != nil {
		panic(err)
	}
	return d
}

func decRef(v string) *decimal.Decimal {
	d := dec(v)
	return &d
}

func (f *ledgerFixture) create(in investments.OperationInput) investments.Operation {
	f.t.Helper()
	op, err := investments.CreateOperation(f.ctx, f.conn, in)
	if err != nil {
		f.t.Fatalf("CreateOperation(%s): %v", in.Kind, err)
	}
	return op
}

func (f *ledgerFixture) deposit(amount string, on time.Time) investments.Operation {
	f.t.Helper()
	return f.create(investments.OperationInput{AccountID: f.custodyID, Kind: investments.OperationDeposit, OccurredOn: on, Amount: dec(amount)})
}

func TestOperationCycleKeepsCashCoherent(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Ações")

	f.deposit("1000", day(1))
	f.assertCash("after deposit", "1000")

	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationBuy,
		OccurredOn: day(2), Amount: dec("600"), Quantity: decRef("6"), UnitPrice: decRef("100"),
		Fees: dec("10"), Taxes: dec("5"),
	})
	f.assertCash("after buy", "385")

	// A valuation restates the holding without touching cash.
	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationValuation,
		OccurredOn: day(3), Amount: dec("700"),
	})
	f.assertCash("after valuation", "385")

	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationSell,
		OccurredOn: day(4), Amount: dec("250"), Quantity: decRef("2"), UnitPrice: decRef("125"),
		Fees: dec("5"), Taxes: dec("2"),
	})
	f.assertCash("after sell", "628")

	f.create(investments.OperationInput{
		AccountID: f.custodyID, Kind: investments.OperationWithdrawal, OccurredOn: day(5), Amount: dec("600"),
	})
	f.assertCash("after withdrawal", "28")

	operations, err := investments.ListOperations(f.ctx, f.conn, investments.OperationFilter{AccountID: &f.custodyID})
	if err != nil {
		t.Fatal(err)
	}
	wantKinds := []investments.OperationKind{
		investments.OperationDeposit, investments.OperationBuy, investments.OperationValuation,
		investments.OperationSell, investments.OperationWithdrawal,
	}
	if len(operations) != len(wantKinds) {
		t.Fatalf("listed %d operations, want %d", len(operations), len(wantKinds))
	}
	for i, kind := range wantKinds {
		if operations[i].Kind != kind {
			t.Fatalf("operation %d = %s, want %s", i, operations[i].Kind, kind)
		}
		if !operations[i].IsEditable || operations[i].Source != "manual" {
			t.Fatalf("operation %d is not manual: %+v", i, operations[i])
		}
	}

	from, to := day(2), day(4)
	ranged, err := investments.ListOperations(f.ctx, f.conn, investments.OperationFilter{From: &from, To: &to})
	if err != nil {
		t.Fatal(err)
	}
	if len(ranged) != 3 || !ranged[0].OccurredOn.Equal(day(2)) || !ranged[2].OccurredOn.Equal(day(4)) {
		t.Fatalf("inclusive range = %+v", ranged)
	}
	byPosition, err := investments.ListOperations(f.ctx, f.conn, investments.OperationFilter{PositionID: &positionID})
	if err != nil {
		t.Fatal(err)
	}
	if len(byPosition) != 3 {
		t.Fatalf("position operations = %d, want 3", len(byPosition))
	}
}

func TestTransferMovesAssetWithoutMovingCash(t *testing.T) {
	f := newLedgerFixture(t)
	destination, err := investments.CreateAccount(f.ctx, f.conn, investments.AccountInput{Name: "Carteira"})
	if err != nil {
		t.Fatal(err)
	}
	ticker := "BTC"
	quantity, cost := dec("1"), dec("100000")
	source, err := investments.CreatePosition(f.ctx, f.conn, investments.PositionInput{AccountID: f.custodyID, Name: "Bitcoin", Ticker: &ticker, AssetType: "Criptoativo", InitialQuantity: quantity, InitialUnitCost: cost, OccurredOn: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	destinationPosition, err := investments.CreatePosition(f.ctx, f.conn, investments.PositionInput{AccountID: destination.ID, AssetID: source.AssetID, Name: "ignored", AssetType: "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	operations, err := investments.CreateTransfer(f.ctx, f.conn, investments.TransferInput{SourcePositionID: source.ID, DestinationPositionID: destinationPosition.ID, Quantity: dec("0.25"), OccurredOn: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 2 || operations[0].TransferID == nil || operations[1].TransferID == nil || *operations[0].TransferID != *operations[1].TransferID {
		t.Fatalf("invalid transfer pair: %#v", operations)
	}
	source, _ = investments.GetPosition(f.ctx, f.conn, source.ID)
	destinationPosition, _ = investments.GetPosition(f.ctx, f.conn, destinationPosition.ID)
	if !source.Quantity.Equal(dec("0.75")) || !destinationPosition.Quantity.Equal(dec("0.25")) {
		t.Fatalf("quantities = %s, %s", source.Quantity, destinationPosition.Quantity)
	}
	accounts, err := investments.ListAccounts(f.ctx, f.conn)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range accounts {
		if (account.ID == f.custodyID || account.ID == destination.ID) && !account.CashBalance.IsZero() {
			t.Fatalf("transfer moved cash in %s: %s", account.ID, account.CashBalance)
		}
	}
}

func TestReinvestedIncomeFundsAPurchase(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "FII")
	f.create(investments.OperationInput{
		AccountID: f.custodyID, Kind: investments.OperationIncome, OccurredOn: day(1), Amount: dec("120.50"),
	})
	f.assertCash("after income", "120.5")
	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationBuy,
		OccurredOn: day(2), Amount: dec("120.50"), Quantity: decRef("1"), UnitPrice: decRef("120.50"),
	})
	f.assertCash("after reinvestment", "0")

	_, err := investments.CreateOperation(f.ctx, f.conn, investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationBuy,
		OccurredOn: day(3), Amount: dec("1"), Quantity: decRef("1"),
	})
	if !errors.Is(err, investments.ErrNegativeCash) {
		t.Fatalf("buy without cash = %v", err)
	}
}

func TestFeesAndTaxesLeaveTheCustodyCash(t *testing.T) {
	f := newLedgerFixture(t)
	f.deposit("1000", day(1))
	f.create(investments.OperationInput{
		AccountID: f.custodyID, Kind: investments.OperationFee, OccurredOn: day(2), Amount: dec("12.30"),
	})
	f.create(investments.OperationInput{
		AccountID: f.custodyID, Kind: investments.OperationTax, OccurredOn: day(3), Amount: dec("7.70"),
	})
	f.assertCash("after costs", "980")
}

func TestSellAboveAvailableQuantityIsRejected(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Ações")
	f.deposit("1000", day(1))
	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationBuy,
		OccurredOn: day(2), Amount: dec("500"), Quantity: decRef("5"), UnitPrice: decRef("100"),
	})

	_, err := investments.CreateOperation(f.ctx, f.conn, investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationSell,
		OccurredOn: day(3), Amount: dec("600"), Quantity: decRef("6"), UnitPrice: decRef("100"),
	})
	if !errors.Is(err, investments.ErrNegativePosition) {
		t.Fatalf("oversized sell = %v", err)
	}
	operations, err := investments.ListOperations(f.ctx, f.conn, investments.OperationFilter{AccountID: &f.custodyID})
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 2 {
		t.Fatalf("rejected sell was persisted: %+v", operations)
	}
	f.assertCash("after rejected sell", "500")
}

func TestHistoricalCorrectionReplaysLaterOperations(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Ações")
	deposit := f.deposit("1000", day(1))
	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationBuy,
		OccurredOn: day(2), Amount: dec("600"), Quantity: decRef("6"), UnitPrice: decRef("100"),
	})
	f.create(investments.OperationInput{
		AccountID: f.custodyID, Kind: investments.OperationWithdrawal, OccurredOn: day(3), Amount: dec("300"),
	})
	f.assertCash("before correction", "100")

	corrected, err := investments.UpdateOperation(f.ctx, f.conn, deposit.ID, investments.OperationInput{
		AccountID: f.custodyID, Kind: investments.OperationDeposit, OccurredOn: day(1), Amount: dec("1200"),
	})
	if err != nil {
		t.Fatalf("UpdateOperation: %v", err)
	}
	if corrected.Amount.String() != "1200" || !corrected.CreatedAt.Equal(deposit.CreatedAt) {
		t.Fatalf("correction = %+v", corrected)
	}
	f.assertCash("after correction", "300")

	// Shrinking the opening deposit would overdraw the custody cash on day 3,
	// which the replay has to refuse without persisting anything.
	_, err = investments.UpdateOperation(f.ctx, f.conn, deposit.ID, investments.OperationInput{
		AccountID: f.custodyID, Kind: investments.OperationDeposit, OccurredOn: day(1), Amount: dec("800"),
	})
	if !errors.Is(err, investments.ErrNegativeCash) {
		t.Fatalf("shrinking correction = %v", err)
	}
	unchanged, err := investments.GetOperation(f.ctx, f.conn, deposit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Amount.String() != "1200" {
		t.Fatalf("rejected correction was persisted: %s", unchanged.Amount)
	}
	f.assertCash("after rejected correction", "300")
}

func TestOperationWritesRejectNonManualTargets(t *testing.T) {
	f := newLedgerFixture(t)
	other, err := investments.CreateAccount(f.ctx, f.conn, investments.AccountInput{Name: "Outra"})
	if err != nil {
		t.Fatal(err)
	}
	foreignPosition := f.addPosition(other.ID, "Fora")
	if _, err := investments.CreateOperation(f.ctx, f.conn, investments.OperationInput{
		AccountID: f.custodyID, PositionID: &foreignPosition, Kind: investments.OperationBuy,
		OccurredOn: day(1), Amount: dec("10"), Quantity: decRef("1"),
	}); !errors.Is(err, investments.ErrInvalidInput) {
		t.Fatalf("foreign position = %v", err)
	}

	syncedID := f.addInvestmentMovement()
	var investmentID string
	if err := f.conn.QueryRow(`SELECT investment_id FROM financial_investment_transactions WHERE id = ?`, syncedID).Scan(&investmentID); err != nil {
		t.Fatal(err)
	}
	if _, err := investments.CreateOperation(f.ctx, f.conn, investments.OperationInput{
		AccountID: f.custodyID, PositionID: &investmentID, Kind: investments.OperationBuy,
		OccurredOn: day(1), Amount: dec("10"), Quantity: decRef("1"),
	}); !errors.Is(err, investments.ErrNotManual) {
		t.Fatalf("synced holding = %v", err)
	}

	// The provider grouping materialized for that connection is read-only.
	if _, err := investments.ListAccounts(f.ctx, f.conn); err != nil {
		t.Fatal(err)
	}
	integratedID := "integrated:" + f.sourceID
	if _, err := investments.CreateOperation(f.ctx, f.conn, investments.OperationInput{
		AccountID: integratedID, Kind: investments.OperationInitialBalance, OccurredOn: day(1), Amount: dec("10"),
	}); !errors.Is(err, investments.ErrIntegratedReadOnly) {
		t.Fatalf("integrated account = %v", err)
	}

	if _, err := investments.GetOperation(f.ctx, f.conn, "missing"); !errors.Is(err, investments.ErrOperationNotFound) {
		t.Fatalf("missing operation = %v", err)
	}
}

func TestSyncedOperationCannotBeChanged(t *testing.T) {
	f := newLedgerFixture(t)
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO investment_operations (id, account_id, kind, occurred_on, amount, source, created_at, updated_at)
		VALUES ('synced-op', ?, 'deposit', '2026-09-01', '100', 'synced', ?, ?)`, f.custodyID, now, now)
	in := investments.OperationInput{AccountID: f.custodyID, Kind: investments.OperationDeposit, OccurredOn: day(1), Amount: dec("200")}
	if _, err := investments.UpdateOperation(f.ctx, f.conn, "synced-op", in); !errors.Is(err, investments.ErrNotManual) {
		t.Fatalf("update = %v", err)
	}
	if err := investments.DeleteOperation(f.ctx, f.conn, "synced-op"); !errors.Is(err, investments.ErrNotManual) {
		t.Fatalf("delete = %v", err)
	}
}

func TestDeleteOperationReplaysRemainingHistory(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Ações")
	deposit := f.deposit("1000", day(1))
	buy := f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationBuy,
		OccurredOn: day(2), Amount: dec("600"), Quantity: decRef("6"), UnitPrice: decRef("100"),
	})

	// Deleting the funding deposit would leave the later purchase overdrawn.
	if err := investments.DeleteOperation(f.ctx, f.conn, deposit.ID); !errors.Is(err, investments.ErrNegativeCash) {
		t.Fatalf("delete of funding deposit = %v", err)
	}
	f.assertCash("after refused delete", "400")

	if err := investments.DeleteOperation(f.ctx, f.conn, buy.ID); err != nil {
		t.Fatal(err)
	}
	f.assertCash("after delete", "1000")
	if err := investments.DeleteOperation(f.ctx, f.conn, buy.ID); !errors.Is(err, investments.ErrOperationNotFound) {
		t.Fatalf("second delete = %v", err)
	}
}

func TestCompoundPurchaseRollbackAndLastQuote(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Ações")
	bankID := f.addBankTransaction("-1000", day(1))
	inputs := []investments.OperationInput{
		{AccountID: f.custodyID, Kind: investments.OperationDeposit, OccurredOn: day(1), Amount: dec("1000")},
		{AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationBuy, OccurredOn: day(1), Amount: dec("1000"), Quantity: decRef("10"), UnitPrice: decRef("100")},
	}
	wrong := f.addBankTransaction("1000", day(1))
	if _, err := investments.CreateOperations(f.ctx, f.conn, inputs, &investments.ReconciliationInput{FinancialTransactionID: &wrong, Amount: dec("1000")}); !errors.Is(err, investments.ErrInvalidReconciliationLink) {
		t.Fatalf("wrong direction = %v", err)
	}
	f.assertCash("rolled back", "0")
	ops, _ := investments.ListOperations(f.ctx, f.conn, investments.OperationFilter{})
	if len(ops) != 0 {
		t.Fatal("orphan operations", ops)
	}
	if _, err := investments.CreateOperations(f.ctx, f.conn, inputs, &investments.ReconciliationInput{FinancialTransactionID: &bankID, Amount: dec("1000")}); err != nil {
		t.Fatal(err)
	}
	f.create(investments.OperationInput{AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationValuation, OccurredOn: day(2), Amount: dec("1200")})
	f.create(investments.OperationInput{AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationSell, OccurredOn: day(3), Amount: dec("480"), Quantity: decRef("4"), UnitPrice: decRef("120")})
	position, err := investments.GetPosition(f.ctx, f.conn, positionID)
	if err != nil {
		t.Fatal(err)
	}
	if position.Quantity.String() != "6" || position.CurrentValue.String() != "720" || position.AverageCost.String() != "100" {
		t.Fatalf("partial sale lost quote or cost: %+v", position)
	}
	f.assertCash("partial sale", "480")
	f.create(investments.OperationInput{AccountID: f.custodyID, Kind: investments.OperationWithdrawal, OccurredOn: day(4), Amount: dec("480")})
	f.assertCash("withdrawal", "0")
}

func TestIntegratedOperationsDoNotChangeProviderAssets(t *testing.T) {
	f := newLedgerFixture(t)
	movement := f.addInvestmentMovement()
	accounts, err := investments.ListAccounts(f.ctx, f.conn)
	if err != nil {
		t.Fatal(err)
	}
	integratedID := "integrated:" + f.sourceID
	if len(accounts) != 2 {
		t.Fatal(accounts)
	}
	before, err := investments.BuildSummary(f.ctx, f.conn)
	if err != nil {
		t.Fatal(err)
	}
	op := f.create(investments.OperationInput{AccountID: integratedID, Kind: investments.OperationDeposit, OccurredOn: day(1), Amount: dec("1000")})
	bankID := f.addBankTransaction("-1000", day(1))
	if _, err := investments.CreateReconciliation(f.ctx, f.conn, investments.ReconciliationInput{OperationID: op.ID, FinancialTransactionID: &bankID, FinancialInvestmentTransactionID: &movement, Amount: dec("1000")}); err != nil {
		t.Fatal(err)
	}
	f.create(investments.OperationInput{AccountID: integratedID, Kind: investments.OperationWithdrawal, OccurredOn: day(2), Amount: dec("500")})
	after, err := investments.BuildSummary(f.ctx, f.conn)
	if err != nil {
		t.Fatal(err)
	}
	if !after.TotalValue.Equal(before.TotalValue) || !after.CashBalance.Equal(before.CashBalance) {
		t.Fatalf("provider assets changed: before=%+v after=%+v", before, after)
	}
}

func TestPostgresInvestmentLedger(t *testing.T) {
	dsn := os.Getenv("CONTADINHO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("CONTADINHO_TEST_POSTGRES_DSN not set")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()
	account, err := investments.CreateAccount(ctx, conn, investments.AccountInput{Name: "Teste de operações"})
	if err != nil {
		t.Fatal(err)
	}
	position, err := investments.CreatePosition(ctx, conn, investments.PositionInput{AccountID: account.ID, Name: "Ativo", AssetType: "Ações"})
	if err != nil {
		t.Fatal(err)
	}
	operations, err := investments.CreateOperations(ctx, conn, []investments.OperationInput{
		{AccountID: account.ID, Kind: investments.OperationDeposit, OccurredOn: day(1), Amount: dec("1000")},
		{AccountID: account.ID, PositionID: &position.ID, Kind: investments.OperationBuy, OccurredOn: day(1), Amount: dec("990"), Fees: dec("10"), Quantity: decRef("10"), UnitPrice: decRef("99")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	position, err = investments.GetPosition(ctx, conn, position.ID)
	if err != nil {
		t.Fatal(err)
	}
	if position.AverageCost.String() != "100" {
		t.Fatal(position)
	}
	if _, err = investments.UpdateOperation(ctx, conn, operations[0].ID, investments.OperationInput{AccountID: account.ID, Kind: investments.OperationDeposit, OccurredOn: day(1), Amount: dec("500")}); !errors.Is(err, investments.ErrNegativeCash) {
		t.Fatalf("correction = %v", err)
	}
}

// TestTransferCarriesCostAsOfItsDate: a transfer dated between two purchases
// takes the cost basis the source holding had on that day, not today's
// blended average.
func TestTransferCarriesCostAsOfItsDate(t *testing.T) {
	f := newLedgerFixture(t)
	destination, err := investments.CreateAccount(f.ctx, f.conn, investments.AccountInput{Name: "Carteira"})
	if err != nil {
		t.Fatal(err)
	}
	ticker := "ABCD3"
	source, err := investments.CreatePosition(f.ctx, f.conn, investments.PositionInput{AccountID: f.custodyID, Name: "Ações", Ticker: &ticker, AssetType: "Ação"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := investments.CreatePosition(f.ctx, f.conn, investments.PositionInput{AccountID: destination.ID, AssetID: source.AssetID, Name: "Ações", AssetType: "Ação"})
	if err != nil {
		t.Fatal(err)
	}
	f.deposit("3000", day(1))
	f.create(investments.OperationInput{AccountID: f.custodyID, PositionID: &source.ID, Kind: investments.OperationBuy, OccurredOn: day(1), Amount: dec("1000"), Quantity: decRef("10"), UnitPrice: decRef("100")})
	f.create(investments.OperationInput{AccountID: f.custodyID, PositionID: &source.ID, Kind: investments.OperationBuy, OccurredOn: day(10), Amount: dec("2000"), Quantity: decRef("10"), UnitPrice: decRef("200")})

	operations, err := investments.CreateTransfer(f.ctx, f.conn, investments.TransferInput{SourcePositionID: source.ID, DestinationPositionID: target.ID, Quantity: dec("10"), OccurredOn: day(5)})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}
	for _, op := range operations {
		if op.Amount.String() != "1000" || op.UnitPrice == nil || op.UnitPrice.String() != "100" {
			t.Fatalf("%s carried amount %s unit %v, want the 100/un cost of 05/09", op.Kind, op.Amount, op.UnitPrice)
		}
	}
	moved, err := investments.GetPosition(f.ctx, f.conn, target.ID)
	if err != nil || moved.Quantity.String() != "10" || moved.CurrentValue.String() != "1000" || bookDecimalString(moved.AverageCost) != "100" {
		t.Fatalf("destination = %+v err=%v", moved, err)
	}
	remaining, err := investments.GetPosition(f.ctx, f.conn, source.ID)
	if err != nil || remaining.Quantity.String() != "10" || remaining.CurrentValue.String() != "2000" || bookDecimalString(remaining.AverageCost) != "200" {
		t.Fatalf("source = %+v err=%v", remaining, err)
	}

	// Moving units the holding did not yet have on the transfer date is
	// refused even though today's quantity would cover it.
	if _, err := investments.CreateTransfer(f.ctx, f.conn, investments.TransferInput{SourcePositionID: source.ID, DestinationPositionID: target.ID, Quantity: dec("5"), OccurredOn: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}); !errors.Is(err, investments.ErrNegativePosition) {
		t.Fatalf("transfer before the units existed = %v, want ErrNegativePosition", err)
	}
}
