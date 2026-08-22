package scenarios_test

import (
	"context"
	"testing"

	"contadinho-go/internal/scenarios"
)

// TestListPlanInstallmentsSkipsRealizedAndOutOfRange covers the two filters
// that make this the timeline's source of payable-plan entries: an
// installment with any realization at all is already carried by the real
// transaction behind that realization, and anything outside the window is not
// this series' business.
func TestListPlanInstallmentsSkipsRealizedAndOutOfRange(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)

	s, err := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	wanted, err := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Maio", dec(t, "300.00"), date(t, "2026-05-10"), nil)
	if err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}
	realized, err := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Junho", dec(t, "300.00"), date(t, "2026-06-10"), nil)
	if err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Dezembro", dec(t, "300.00"), date(t, "2026-12-10"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}
	link := linkFixture(t, conn, d.ID, "-100.00")
	// A partial realization is still a realization: the real transaction is
	// already in the series, so the installment must not be projected too.
	if _, err := scenarios.CreateRealization(ctx, conn, realized.ID, &link.ID, dec(t, "100.00")); err != nil {
		t.Fatalf("CreateRealization: %v", err)
	}

	got, err := scenarios.ListPlanInstallments(ctx, conn, date(t, "2026-05-01"), date(t, "2026-06-30"))
	if err != nil {
		t.Fatalf("ListPlanInstallments: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d installments, want only May's: %+v", len(got), got)
	}
	if got[0].TransactionID != wanted.ID {
		t.Errorf("TransactionID = %s, want May's %s", got[0].TransactionID, wanted.ID)
	}
	if got[0].PayableID != d.ID || got[0].ScenarioKind != scenarios.KindDebtPlan {
		t.Errorf("got %+v, want the backing payable and plan kind carried through", got[0])
	}
	if got[0].Amount.String() != "-300" {
		t.Errorf("Amount = %s, want -300 — a debt plan's installment is money leaving", got[0].Amount)
	}
	if !got[0].ProjectedAt.Equal(date(t, "2026-05-10")) {
		t.Errorf("ProjectedAt = %v, want 2026-05-10", got[0].ProjectedAt)
	}
}

// TestListPlanInstallmentsBoundsAreInclusiveDays pins that the window is
// compared as calendar days, endpoints included — a caller passing a
// timestamp gets its day, not a partial-day cutoff.
func TestListPlanInstallmentsBoundsAreInclusiveDays(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)

	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)
	for _, day := range []string{"2026-05-01", "2026-05-31"} {
		if _, err := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, day, dec(t, "100.00"), date(t, day), nil); err != nil {
			t.Fatalf("CreateScenarioTransaction %s: %v", day, err)
		}
	}

	got, err := scenarios.ListPlanInstallments(ctx, conn, date(t, "2026-05-01"), date(t, "2026-05-31"))
	if err != nil {
		t.Fatalf("ListPlanInstallments: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d installments, want both endpoints included: %+v", len(got), got)
	}
}

// TestListPlanInstallmentsIgnoresStandaloneScenarios pins the boundary
// between the two scenario flavours: a standalone "what if" is hypothetical
// and only enters a series when the caller selects it by id, never through
// the payable-plan path.
func TestListPlanInstallmentsIgnoresStandaloneScenarios(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	s, err := scenarios.CreateScenario(ctx, conn, scenarios.KindStandalone, "Viagem", nil)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Passagens", dec(t, "-2000.00"), date(t, "2026-05-10"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}

	got, err := scenarios.ListPlanInstallments(ctx, conn, date(t, "2026-01-01"), date(t, "2026-12-31"))
	if err != nil {
		t.Fatalf("ListPlanInstallments: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want none — a standalone scenario is not a payable plan", got)
	}
}

// TestListPlanInstallmentsCarriesReceivableDirection is the mirror of the
// debt case: a receivable plan's installment is money arriving.
func TestListPlanInstallmentsCarriesReceivableDirection(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	r := newReceivable(t, conn)

	s, err := scenarios.CreateScenario(ctx, conn, scenarios.KindReceivablePlan, "Cobrança", &r.ID)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-05-10"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}

	got, err := scenarios.ListPlanInstallments(ctx, conn, date(t, "2026-05-01"), date(t, "2026-05-31"))
	if err != nil {
		t.Fatalf("ListPlanInstallments: %v", err)
	}
	if len(got) != 1 || got[0].Amount.String() != "400" {
		t.Fatalf("got %+v, want a single +400 inflow", got)
	}
}
