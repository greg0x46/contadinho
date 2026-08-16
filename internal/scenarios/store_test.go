package scenarios_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/payables"
	"contadinho-go/internal/scenarios"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func newDebt(t *testing.T, conn *sql.DB) payables.Payable {
	t.Helper()
	d, err := payables.Create(context.Background(), conn, payables.KindDebt, "Cartão de crédito", dec(t, "1200.00"), dec(t, "1200.00"))
	if err != nil {
		t.Fatalf("payables.Create: %v", err)
	}
	return d
}

func TestCreateGetListDeleteScenario(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)

	s, err := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano de pagamento", &d.ID)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if s.Kind != scenarios.KindDebtPlan || s.PayableID == nil || *s.PayableID != d.ID {
		t.Fatalf("CreateScenario() = %+v", s)
	}

	got, err := scenarios.GetScenario(ctx, conn, s.ID)
	if err != nil {
		t.Fatalf("GetScenario: %v", err)
	}
	if got.Name != "Plano de pagamento" {
		t.Errorf("GetScenario() = %+v", got)
	}

	list, err := scenarios.ListScenariosByPayable(ctx, conn, d.ID)
	if err != nil {
		t.Fatalf("ListScenariosByPayable: %v", err)
	}
	if len(list) != 1 || list[0].ID != s.ID {
		t.Errorf("ListScenariosByPayable() = %+v", list)
	}

	if err := scenarios.DeleteScenario(ctx, conn, s.ID); err != nil {
		t.Fatalf("DeleteScenario: %v", err)
	}
	if _, err := scenarios.GetScenario(ctx, conn, s.ID); !errors.Is(err, scenarios.ErrScenarioNotFound) {
		t.Errorf("GetScenario after delete: err = %v, want ErrScenarioNotFound", err)
	}
}

func TestGetUnknownScenario(t *testing.T) {
	conn := newTestDB(t)
	if _, err := scenarios.GetScenario(context.Background(), conn, "unknown"); !errors.Is(err, scenarios.ErrScenarioNotFound) {
		t.Errorf("GetScenario: err = %v, want ErrScenarioNotFound", err)
	}
}

func TestDebtPlanScenarioRequiresPayableID(t *testing.T) {
	conn := newTestDB(t)
	if _, err := scenarios.CreateScenario(context.Background(), conn, scenarios.KindDebtPlan, "Sem dívida", nil); err == nil {
		t.Error("CreateScenario() with no payable_id should fail the CHECK constraint")
	}
}

func TestStandaloneScenarioRejectsPayableID(t *testing.T) {
	conn := newTestDB(t)
	d := newDebt(t, conn)
	if _, err := scenarios.CreateScenario(context.Background(), conn, scenarios.KindStandalone, "Viagem", &d.ID); err == nil {
		t.Error("CreateScenario(standalone) with a payable_id should fail the CHECK constraint")
	}
}

func TestListStandaloneScenarios(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	if _, err := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID); err != nil {
		t.Fatalf("CreateScenario(debt_plan): %v", err)
	}
	standalone, err := scenarios.CreateScenario(ctx, conn, scenarios.KindStandalone, "Viagem", nil)
	if err != nil {
		t.Fatalf("CreateScenario(standalone): %v", err)
	}

	list, err := scenarios.ListStandaloneScenarios(ctx, conn)
	if err != nil {
		t.Fatalf("ListStandaloneScenarios: %v", err)
	}
	if len(list) != 1 || list[0].ID != standalone.ID {
		t.Errorf("ListStandaloneScenarios() = %+v, want only %+v", list, standalone)
	}
}

func TestListScenariosByIDs(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	a, err := scenarios.CreateScenario(ctx, conn, scenarios.KindStandalone, "Viagem", nil)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	b, err := scenarios.CreateScenario(ctx, conn, scenarios.KindStandalone, "Novo emprego", nil)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenario(ctx, conn, scenarios.KindStandalone, "Não incluído", nil); err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}

	list, err := scenarios.ListScenariosByIDs(ctx, conn, []string{a.ID, b.ID})
	if err != nil {
		t.Fatalf("ListScenariosByIDs: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListScenariosByIDs() = %+v, want 2 scenarios", list)
	}

	empty, err := scenarios.ListScenariosByIDs(ctx, conn, nil)
	if err != nil {
		t.Fatalf("ListScenariosByIDs(nil): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("ListScenariosByIDs(nil) = %+v, want empty", empty)
	}
}

func TestCreateGetUpdateDeleteScenarioTransaction(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)

	category := "Dívidas"
	st, err := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), &category)
	if err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}
	if !st.Amount.Equal(dec(t, "400.00")) || !st.ProjectedAt.Equal(date(t, "2026-09-01")) {
		t.Fatalf("CreateScenarioTransaction() = %+v", st)
	}

	got, err := scenarios.GetScenarioTransaction(ctx, conn, st.ID)
	if err != nil {
		t.Fatalf("GetScenarioTransaction: %v", err)
	}
	if got.Description != "Parcela 1" || got.Category == nil || *got.Category != "Dívidas" {
		t.Errorf("GetScenarioTransaction() = %+v", got)
	}

	updated, err := scenarios.UpdateScenarioTransaction(ctx, conn, st.ID, "Parcela 1 (editada)", dec(t, "450.00"), date(t, "2026-09-05"), nil)
	if err != nil {
		t.Fatalf("UpdateScenarioTransaction: %v", err)
	}
	if updated.Description != "Parcela 1 (editada)" || !updated.Amount.Equal(dec(t, "450.00")) || updated.Category != nil {
		t.Errorf("UpdateScenarioTransaction() = %+v", updated)
	}

	if err := scenarios.DeleteScenarioTransaction(ctx, conn, st.ID); err != nil {
		t.Fatalf("DeleteScenarioTransaction: %v", err)
	}
	if _, err := scenarios.GetScenarioTransaction(ctx, conn, st.ID); !errors.Is(err, scenarios.ErrTransactionNotFound) {
		t.Errorf("GetScenarioTransaction after delete: err = %v, want ErrTransactionNotFound", err)
	}
}

func TestUpdateDeleteUnknownScenarioTransaction(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	if _, err := scenarios.UpdateScenarioTransaction(ctx, conn, "unknown", "x", dec(t, "1.00"), date(t, "2026-01-01"), nil); !errors.Is(err, scenarios.ErrTransactionNotFound) {
		t.Errorf("UpdateScenarioTransaction: err = %v, want ErrTransactionNotFound", err)
	}
	if err := scenarios.DeleteScenarioTransaction(ctx, conn, "unknown"); !errors.Is(err, scenarios.ErrTransactionNotFound) {
		t.Errorf("DeleteScenarioTransaction: err = %v, want ErrTransactionNotFound", err)
	}
}

func TestListScenarioTransactionsOrderedByProjectedAt(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)

	third, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 3", dec(t, "400.00"), date(t, "2026-11-01"), nil)
	first, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), nil)
	second, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 2", dec(t, "400.00"), date(t, "2026-10-01"), nil)

	list, err := scenarios.ListScenarioTransactions(ctx, conn, s.ID)
	if err != nil {
		t.Fatalf("ListScenarioTransactions: %v", err)
	}
	if len(list) != 3 || list[0].ID != first.ID || list[1].ID != second.ID || list[2].ID != third.ID {
		t.Fatalf("ListScenarioTransactions() not ordered by projected_at: %+v", list)
	}
}

func TestDeleteScenarioCascadesTransactions(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)
	st, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), nil)

	if err := scenarios.DeleteScenario(ctx, conn, s.ID); err != nil {
		t.Fatalf("DeleteScenario: %v", err)
	}
	if _, err := scenarios.GetScenarioTransaction(ctx, conn, st.ID); !errors.Is(err, scenarios.ErrTransactionNotFound) {
		t.Errorf("GetScenarioTransaction after scenario delete: err = %v, want ErrTransactionNotFound", err)
	}
}

func TestDeleteDebtCascadesScenario(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)

	if err := payables.Delete(ctx, conn, d.ID); err != nil {
		t.Fatalf("payables.Delete: %v", err)
	}
	if _, err := scenarios.GetScenario(ctx, conn, s.ID); !errors.Is(err, scenarios.ErrScenarioNotFound) {
		t.Errorf("GetScenario after debt delete: err = %v, want ErrScenarioNotFound", err)
	}
}
