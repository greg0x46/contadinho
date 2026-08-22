package projections_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/payables"
	"contadinho-go/internal/projections"
	"contadinho-go/internal/recurrences"
	"contadinho-go/internal/scenarios"
)

const projectionCategoryID = "000433b6-3094-5a9c-87df-465b70574a4b"

func projectionDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func projectionDecimal(t *testing.T, value string) decimal.Decimal {
	t.Helper()
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func projectionDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestListSelectionAndEventKeys(t *testing.T) {
	conn := projectionDB(t)
	ctx := context.Background()
	commitment, err := recurrences.Create(ctx, conn, recurrences.Write{
		Name: "Aluguel", Kind: recurrences.KindExpense, Amount: projectionDecimal(t, "100.00"),
		CategoryID: projectionCategoryID, Cadence: recurrences.CadenceMonthly, DayOfMonth: 15,
		StartDate: projectionDate(t, "2026-01-01"), IsActive: true,
	})
	if err != nil {
		t.Fatalf("recurrences.Create: %v", err)
	}
	if commitment.ScenarioID == nil {
		t.Fatal("recurring compatibility DTO must expose its canonical scenario id")
	}

	query := projections.ProjectionQuery{
		From: projectionDate(t, "2026-01-01"), To: projectionDate(t, "2026-04-30"),
		Selection: projections.SelectionActive,
	}
	active, err := projections.List(ctx, conn, query)
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if len(active) != 4 {
		t.Fatalf("active projections = %d, want 4", len(active))
	}
	firstKeys := make([]string, len(active))
	for i, event := range active {
		firstKeys[i] = event.EventKey
		want := "scenario:" + *commitment.ScenarioID + ":occurrence:" + event.Date.Format("2006-01-02")
		if event.EventKey != want {
			t.Errorf("event key = %q, want %q", event.EventKey, want)
		}
	}

	if _, err := recurrences.SetActive(ctx, conn, commitment.ID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	active, err = projections.List(ctx, conn, query)
	if err != nil {
		t.Fatalf("List inactive active-selection: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("inactive scenario emitted %d active projections", len(active))
	}

	explicit := query
	explicit.Selection = projections.SelectionExplicit
	explicit.IDs = []string{*commitment.ScenarioID}
	selected, err := projections.List(ctx, conn, explicit)
	if err != nil {
		t.Fatalf("List explicit inactive: %v", err)
	}
	if len(selected) != 4 {
		t.Fatalf("explicit inactive projections = %d, want 4", len(selected))
	}
	for i, event := range selected {
		if event.EventKey != firstKeys[i] {
			t.Errorf("event key changed after deactivation at %d: got %q, want %q", i, event.EventKey, firstKeys[i])
		}
	}
}

func TestListReturnsUnifiedKindsAndRealizationState(t *testing.T) {
	conn := projectionDB(t)
	ctx := context.Background()

	standalone, err := scenarios.CreateScenario(ctx, conn, scenarios.KindStandalone, "Viagem", nil)
	if err != nil {
		t.Fatalf("Create standalone: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, conn, standalone.ID, "Passagem", projectionDecimal(t, "-300.00"), projectionDate(t, "2026-02-10"), nil); err != nil {
		t.Fatalf("Create standalone transaction: %v", err)
	}

	payable, err := payables.Create(ctx, conn, payables.KindDebt, "Financiamento", projectionDecimal(t, "500.00"), projectionDecimal(t, "500.00"))
	if err != nil {
		t.Fatalf("payables.Create: %v", err)
	}
	plan, err := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &payable.ID)
	if err != nil {
		t.Fatalf("Create plan: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, conn, plan.ID, "Parcela", projectionDecimal(t, "100.00"), projectionDate(t, "2026-02-11"), nil); err != nil {
		t.Fatalf("Create plan transaction: %v", err)
	}

	query := projections.ProjectionQuery{
		From: projectionDate(t, "2026-02-01"), To: projectionDate(t, "2026-02-28"),
		Selection: projections.SelectionExplicit, IDs: []string{standalone.ID, plan.ID},
	}
	events, err := projections.List(ctx, conn, query)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want one standalone and one plan", events)
	}
	if events[0].ScenarioKind != scenarios.KindStandalone || string(events[0].Tier) != "hipotetico" {
		t.Errorf("standalone event = %+v", events[0])
	}
	if events[1].ScenarioKind != scenarios.KindDebtPlan || events[1].Amount.String() != "-100" || string(events[1].Tier) != "confirmado" {
		t.Errorf("plan event = %+v", events[1])
	}

	commitment, err := recurrences.Create(ctx, conn, recurrences.Write{
		Name: "Salário", Kind: recurrences.KindIncome, Amount: projectionDecimal(t, "200.00"),
		CategoryID: projectionCategoryID, Cadence: recurrences.CadenceMonthly, DayOfMonth: 20,
		StartDate: projectionDate(t, "2026-02-01"), IsActive: true,
	})
	if err != nil {
		t.Fatalf("recurrences.Create: %v", err)
	}
	recurringQuery := projections.ProjectionQuery{
		From: projectionDate(t, "2026-02-01"), To: projectionDate(t, "2026-02-28"),
		Selection: projections.SelectionExplicit, IDs: []string{*commitment.ScenarioID},
	}
	events, err = projections.List(ctx, conn, recurringQuery)
	if err != nil {
		t.Fatalf("List recurring: %v", err)
	}
	if len(events) != 1 || events[0].Amount.String() != "200" || events[0].ScenarioKind != scenarios.KindRecurring {
		t.Fatalf("recurring event = %+v", events)
	}

	if _, err := scenarios.RealizeEvent(ctx, conn, *commitment.ScenarioID, events[0].EventKey, scenarios.RealizationWrite{
		State: scenarios.RealizationStateDetached,
	}); err != nil {
		t.Fatalf("detach occurrence: %v", err)
	}
	events, err = projections.List(ctx, conn, recurringQuery)
	if err != nil {
		t.Fatalf("List detached: %v", err)
	}
	if len(events) != 1 || !events[0].Detached || events[0].Realized {
		t.Fatalf("detached event = %+v", events)
	}
}

func TestLinkedOccurrenceIsMarkedRealized(t *testing.T) {
	conn := projectionDB(t)
	ctx := context.Background()
	commitment, err := recurrences.Create(ctx, conn, recurrences.Write{
		Name: "Salário", Kind: recurrences.KindIncome, Amount: projectionDecimal(t, "200.00"),
		CategoryID: projectionCategoryID, Cadence: recurrences.CadenceMonthly, DayOfMonth: 20,
		StartDate: projectionDate(t, "2026-02-01"), IsActive: true,
	})
	if err != nil {
		t.Fatalf("recurrences.Create: %v", err)
	}
	txID := insertProjectionTransaction(t, conn)
	eventKey := "scenario:" + *commitment.ScenarioID + ":occurrence:2026-02-20"
	if _, err := scenarios.RealizeEvent(ctx, conn, *commitment.ScenarioID, eventKey, scenarios.RealizationWrite{
		State: scenarios.RealizationStateLinked, TransactionID: &txID,
	}); err != nil {
		t.Fatalf("link occurrence: %v", err)
	}

	events, err := projections.List(ctx, conn, projections.ProjectionQuery{
		From: projectionDate(t, "2026-02-01"), To: projectionDate(t, "2026-02-28"),
		Selection: projections.SelectionExplicit, IDs: []string{*commitment.ScenarioID},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 1 || !events[0].Realized || events[0].RealizationOrigin != "manual" {
		t.Fatalf("linked projection = %+v", events)
	}
}

func TestSameTransactionCannotReconcileTwoOccurrences(t *testing.T) {
	conn := projectionDB(t)
	ctx := context.Background()
	makeCommitment := func(name string) recurrences.RecurringCommitment {
		commitment, err := recurrences.Create(ctx, conn, recurrences.Write{
			Name: name, Kind: recurrences.KindIncome, Amount: projectionDecimal(t, "200.00"),
			CategoryID: projectionCategoryID, Cadence: recurrences.CadenceMonthly, DayOfMonth: 20,
			StartDate: projectionDate(t, "2026-02-01"), IsActive: true,
		})
		if err != nil {
			t.Fatalf("recurrences.Create: %v", err)
		}
		return commitment
	}
	first := makeCommitment("Salário A")
	second := makeCommitment("Salário B")
	txID := insertProjectionTransaction(t, conn)
	firstKey := "scenario:" + *first.ScenarioID + ":occurrence:2026-02-20"
	secondKey := "scenario:" + *second.ScenarioID + ":occurrence:2026-02-20"
	if _, err := scenarios.RealizeEvent(ctx, conn, *first.ScenarioID, firstKey, scenarios.RealizationWrite{
		State: scenarios.RealizationStateLinked, TransactionID: &txID,
	}); err != nil {
		t.Fatalf("link first occurrence: %v", err)
	}
	if _, err := scenarios.RealizeEvent(ctx, conn, *second.ScenarioID, secondKey, scenarios.RealizationWrite{
		State: scenarios.RealizationStateLinked, TransactionID: &txID,
	}); !errors.Is(err, scenarios.ErrTransactionAlreadyRealized) {
		t.Fatalf("second link error = %v, want ErrTransactionAlreadyRealized", err)
	}
}

func TestGenericRecurringRealizationRejectsWrongDirection(t *testing.T) {
	conn := projectionDB(t)
	ctx := context.Background()
	commitment, err := recurrences.Create(ctx, conn, recurrences.Write{
		Name: "Aluguel", Kind: recurrences.KindExpense, Amount: projectionDecimal(t, "200.00"),
		CategoryID: projectionCategoryID, Cadence: recurrences.CadenceMonthly, DayOfMonth: 20,
		StartDate: projectionDate(t, "2026-02-01"), IsActive: true,
	})
	if err != nil {
		t.Fatalf("recurrences.Create: %v", err)
	}
	txID := insertProjectionTransaction(t, conn) // positive CREDIT, not an expense
	_, err = scenarios.RealizeEvent(ctx, conn, *commitment.ScenarioID,
		"scenario:"+*commitment.ScenarioID+":occurrence:2026-02-20", scenarios.RealizationWrite{
			State: scenarios.RealizationStateLinked, TransactionID: &txID,
		})
	if !errors.Is(err, scenarios.ErrInvalidRealization) {
		t.Fatalf("wrong-direction realization error = %v, want ErrInvalidRealization", err)
	}
}

func TestGenericRealizationRejectsNonEventAndUnlinkedPlanTransaction(t *testing.T) {
	conn := projectionDB(t)
	ctx := context.Background()

	commitment, err := recurrences.Create(ctx, conn, recurrences.Write{
		Name: "Salário", Kind: recurrences.KindIncome, Amount: projectionDecimal(t, "200.00"),
		CategoryID: projectionCategoryID, Cadence: recurrences.CadenceMonthly, DayOfMonth: 20,
		StartDate: projectionDate(t, "2026-02-01"), IsActive: true,
	})
	if err != nil {
		t.Fatalf("recurrences.Create: %v", err)
	}
	if _, err := scenarios.RealizeEvent(ctx, conn, *commitment.ScenarioID,
		"scenario:"+*commitment.ScenarioID+":occurrence:2026-02-21", scenarios.RealizationWrite{
			State: scenarios.RealizationStateDetached,
		}); !errors.Is(err, scenarios.ErrInvalidEventKey) {
		t.Fatalf("invalid occurrence error = %v, want ErrInvalidEventKey", err)
	}

	payable, err := payables.Create(ctx, conn, payables.KindDebt, "Financiamento", projectionDecimal(t, "500.00"), projectionDecimal(t, "500.00"))
	if err != nil {
		t.Fatalf("payables.Create: %v", err)
	}
	plan, err := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &payable.ID)
	if err != nil {
		t.Fatalf("Create plan: %v", err)
	}
	planned, err := scenarios.CreateScenarioTransaction(ctx, conn, plan.ID, "Parcela", projectionDecimal(t, "100.00"), projectionDate(t, "2026-02-11"), nil)
	if err != nil {
		t.Fatalf("Create plan transaction: %v", err)
	}
	txID := insertProjectionTransaction(t, conn)
	if _, err := scenarios.RealizeEvent(ctx, conn, plan.ID,
		"scenario:"+plan.ID+":transaction:"+planned.ID, scenarios.RealizationWrite{
			State: scenarios.RealizationStateLinked, TransactionID: &txID,
		}); !errors.Is(err, scenarios.ErrInvalidRealization) {
		t.Fatalf("unlinked plan transaction error = %v, want ErrInvalidRealization", err)
	}
}

func insertProjectionTransaction(t *testing.T, conn *sql.DB) string {
	t.Helper()
	now := db.FormatTime(time.Now().UTC())
	sourceID, runID, importID, accountID, txID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at) VALUES (?, 'pluggy', ?, ?, ?)`, sourceID, sourceID, now, now)
	exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at) VALUES (?, ?, 'completed', ?, ?)`, runID, sourceID, now, now)
	exec(`INSERT INTO raw_imports (id, sync_run_id, source_id, scope, page_sequence, request_attempt, request_method, request_path, http_status, response_headers, payload, payload_sha256, received_at) VALUES (?, ?, ?, 'transactions', 1, 1, 'GET', '/x', 200, '{}', x'00', 'hash', ?)`, importID, runID, sourceID, now)
	exec(`INSERT INTO financial_accounts (id, source_id, external_id, currency_code, current_raw_import_id, normalized_hash, created_at, updated_at) VALUES (?, ?, ?, 'BRL', ?, 'hash', ?, ?)`, accountID, sourceID, accountID, importID, now, now)
	exec(`INSERT INTO financial_transactions (id, source_id, account_id, external_id, description, amount, amount_in_account_currency, currency_code, occurred_at, provider_status, movement_type, current_raw_import_id, normalized_hash, created_at, updated_at) VALUES (?, ?, ?, ?, 'Salário', '200.00', '200.00', 'BRL', '2026-02-20T00:00:00.000000000Z', 'POSTED', 'CREDIT', ?, 'hash', ?, ?)`, txID, sourceID, accountID, txID, importID, now, now)
	return txID
}
