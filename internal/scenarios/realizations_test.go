package scenarios_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
	"contadinho-go/internal/debts"
	"contadinho-go/internal/scenarios"
)

// linkFixture creates the minimal sync-schema chain plus one
// financial_transactions row and links it to debtID, mirroring
// internal/debts/store_test.go's fixture (not importable here since it's
// unexported in an external test package).
func linkFixture(t *testing.T, conn *sql.DB, debtID, amount string) debts.Link {
	t.Helper()
	ctx := context.Background()
	now := db.FormatTime(time.Now())

	sourceID := uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at) VALUES (?, 'pluggy', ?, ?, ?)`,
		sourceID, "item-"+sourceID, now, now)
	syncRunID := uuid.NewString()
	exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at) VALUES (?, ?, 'completed', ?, ?)`,
		syncRunID, sourceID, now, now)
	rawImportID := uuid.NewString()
	exec(`INSERT INTO raw_imports (
			id, sync_run_id, source_id, scope, page_sequence, request_attempt,
			request_method, request_path, http_status, response_headers, payload,
			payload_sha256, received_at
		) VALUES (?, ?, ?, 'transactions', 1, 1, 'GET', '/x', 200, '{}', x'00', 'sha', ?)`,
		rawImportID, syncRunID, sourceID, now)
	accountID := uuid.NewString()
	exec(`INSERT INTO financial_accounts (
			id, source_id, external_id, currency_code, current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, 'BRL', ?, 'hash', ?, ?)`, accountID, sourceID, accountID, rawImportID, now, now)
	txID := uuid.NewString()
	exec(`INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description, amount, amount_in_account_currency,
			currency_code, occurred_at, provider_status, movement_type, current_raw_import_id,
			normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'Pagamento', ?, ?, 'BRL', ?, 'POSTED', 'DEBIT', ?, 'hash', ?, ?)`,
		txID, sourceID, accountID, txID, amount, amount, now, rawImportID, now, now)

	result, err := debts.CreateLink(ctx, conn, debtID, txID)
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if result.Status != debts.StatusCreated {
		t.Fatalf("CreateLink status = %s", result.Status)
	}
	return *result.Link
}

func TestCreateRealizationAndRealizedTotal(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID, nil)
	st, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), nil)

	link1 := linkFixture(t, conn, d.ID, "-150.00")
	link2 := linkFixture(t, conn, d.ID, "-250.00")

	if _, err := scenarios.CreateRealization(ctx, conn, st.ID, &link1.ID, nil, dec(t, "150.00")); err != nil {
		t.Fatalf("CreateRealization: %v", err)
	}
	if _, err := scenarios.CreateRealization(ctx, conn, st.ID, &link2.ID, nil, dec(t, "250.00")); err != nil {
		t.Fatalf("CreateRealization: %v", err)
	}

	total, err := scenarios.RealizedTotal(ctx, conn, st.ID)
	if err != nil {
		t.Fatalf("RealizedTotal: %v", err)
	}
	if !total.Equal(dec(t, "400.00")) {
		t.Errorf("RealizedTotal = %s, want 400.00", total)
	}

	realizations, err := scenarios.ListRealizationsForTransaction(ctx, conn, st.ID)
	if err != nil {
		t.Fatalf("ListRealizationsForTransaction: %v", err)
	}
	if len(realizations) != 2 {
		t.Fatalf("len(realizations) = %d, want 2", len(realizations))
	}
}

func TestRealizedTotalIsZeroWithNoAllocations(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID, nil)
	st, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), nil)

	total, err := scenarios.RealizedTotal(ctx, conn, st.ID)
	if err != nil {
		t.Fatalf("RealizedTotal: %v", err)
	}
	if !total.IsZero() {
		t.Errorf("RealizedTotal = %s, want 0", total)
	}
}

func TestDeleteRealization(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID, nil)
	st, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), nil)
	link := linkFixture(t, conn, d.ID, "-400.00")
	realization, err := scenarios.CreateRealization(ctx, conn, st.ID, &link.ID, nil, dec(t, "400.00"))
	if err != nil {
		t.Fatalf("CreateRealization: %v", err)
	}

	if err := scenarios.DeleteRealization(ctx, conn, realization.ID); err != nil {
		t.Fatalf("DeleteRealization: %v", err)
	}
	total, err := scenarios.RealizedTotal(ctx, conn, st.ID)
	if err != nil {
		t.Fatalf("RealizedTotal: %v", err)
	}
	if !total.IsZero() {
		t.Errorf("RealizedTotal after delete = %s, want 0", total)
	}
	if err := scenarios.DeleteRealization(ctx, conn, realization.ID); !errors.Is(err, scenarios.ErrRealizationNotFound) {
		t.Errorf("DeleteRealization (again): err = %v, want ErrRealizationNotFound", err)
	}
}

func TestScenarioTransactionStatusFromRealizedTotal(t *testing.T) {
	st := scenarios.ScenarioTransaction{Amount: dec(t, "400.00"), ProjectedAt: date(t, "2026-01-01")}
	today := date(t, "2026-06-01")

	cases := []struct {
		realizedTotal string
		want          scenarios.Status
	}{
		{"0", scenarios.StatusAtrasada},
		{"200.00", scenarios.StatusPagaParcialmente},
		{"400.00", scenarios.StatusPaga},
		{"500.00", scenarios.StatusPagaAMais},
	}
	for _, c := range cases {
		got := st.Status(today, dec(t, c.realizedTotal))
		if got != c.want {
			t.Errorf("Status(realizedTotal=%s) = %s, want %s", c.realizedTotal, got, c.want)
		}
	}
}

func TestCreateRealizationRejectsUnknownDebtLink(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID, nil)
	st, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), nil)

	unknownLink := "unknown-link"
	if _, err := scenarios.CreateRealization(ctx, conn, st.ID, &unknownLink, nil, dec(t, "100.00")); err == nil {
		t.Error("CreateRealization() with an unknown debt_link_id should fail the FK constraint")
	}
}
