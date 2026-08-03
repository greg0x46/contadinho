package scenarios_test

import (
	"context"
	"errors"
	"testing"

	"contadinho-go/internal/scenarios"
)

func TestReadjustReduceTermRecomputesCountFromReferenceAmount(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)

	// Three 400.00 installments (1200.00 total), none allocated.
	drafts, _ := scenarios.GenerateInstallments(dec(t, "1200.00"), 3, date(t, "2026-09-01"), scenarios.CadenceMonthly)
	if _, err := scenarios.CreateGeneratedInstallments(ctx, conn, s.ID, drafts); err != nil {
		t.Fatalf("CreateGeneratedInstallments: %v", err)
	}

	// Remaining amount dropped to 900.00 (e.g. paid down outside the plan);
	// abater do final should keep the 400.00 reference value and need
	// ceil(900/400) = 3 installments (400, 400, 100).
	created, err := scenarios.Readjust(ctx, conn, s.ID, dec(t, "900.00"), scenarios.StrategyReduceTerm)
	if err != nil {
		t.Fatalf("Readjust: %v", err)
	}
	if len(created) != 3 {
		t.Fatalf("len(created) = %d, want 3", len(created))
	}
	sum := dec(t, "0")
	for i, ins := range created {
		sum = sum.Add(ins.Amount)
		if i < 2 && !ins.Amount.Equal(dec(t, "400.00")) {
			t.Errorf("installment[%d] = %s, want 400.00", i, ins.Amount)
		}
	}
	if !created[2].Amount.Equal(dec(t, "100.00")) {
		t.Errorf("last installment = %s, want 100.00 (absorbs remainder)", created[2].Amount)
	}
	if !sum.Equal(dec(t, "900.00")) {
		t.Errorf("sum = %s, want 900.00", sum)
	}
	if !created[0].ProjectedAt.Equal(date(t, "2026-09-01")) {
		t.Errorf("first installment date = %s, want 2026-09-01 (starts at earliest affected date)", created[0].ProjectedAt)
	}

	remaining, err := scenarios.ListScenarioTransactions(ctx, conn, s.ID)
	if err != nil {
		t.Fatalf("ListScenarioTransactions: %v", err)
	}
	if len(remaining) != 3 {
		t.Fatalf("len(remaining) = %d, want 3 (old ones replaced, not appended)", len(remaining))
	}
}

func TestReadjustReduceTermTwiceDoesNotLockOntoRemainderInstallment(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)

	// Three 400.00 installments (1200.00 total), none allocated.
	drafts, _ := scenarios.GenerateInstallments(dec(t, "1200.00"), 3, date(t, "2026-09-01"), scenarios.CadenceMonthly)
	if _, err := scenarios.CreateGeneratedInstallments(ctx, conn, s.ID, drafts); err != nil {
		t.Fatalf("CreateGeneratedInstallments: %v", err)
	}

	// First readjust: 900.00 -> [400, 400, 100], per the other reduce-term
	// test. The 100.00 is a remainder, not the plan's real installment
	// value.
	if _, err := scenarios.Readjust(ctx, conn, s.ID, dec(t, "900.00"), scenarios.StrategyReduceTerm); err != nil {
		t.Fatalf("first Readjust: %v", err)
	}

	// Readjusting again for the same 900.00 (nothing paid, nothing changed)
	// must reproduce the same [400, 400, 100] plan, not lock onto the
	// 100.00 remainder as the reference amount and explode the count
	// (ceil(900/100) = 9).
	created, err := scenarios.Readjust(ctx, conn, s.ID, dec(t, "900.00"), scenarios.StrategyReduceTerm)
	if err != nil {
		t.Fatalf("second Readjust: %v", err)
	}
	if len(created) != 3 {
		t.Fatalf("len(created) = %d, want 3 (must not lock onto the remainder installment as the reference amount)", len(created))
	}
	sum := dec(t, "0")
	for _, ins := range created {
		sum = sum.Add(ins.Amount)
	}
	if !sum.Equal(dec(t, "900.00")) {
		t.Errorf("sum = %s, want 900.00", sum)
	}
}

func TestReadjustRedistributePreservesInstallmentCountAndDates(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)

	drafts, _ := scenarios.GenerateInstallments(dec(t, "1200.00"), 3, date(t, "2026-09-01"), scenarios.CadenceMonthly)
	if _, err := scenarios.CreateGeneratedInstallments(ctx, conn, s.ID, drafts); err != nil {
		t.Fatalf("CreateGeneratedInstallments: %v", err)
	}

	created, err := scenarios.Readjust(ctx, conn, s.ID, dec(t, "900.00"), scenarios.StrategyRedistribute)
	if err != nil {
		t.Fatalf("Readjust: %v", err)
	}
	if len(created) != 3 {
		t.Fatalf("len(created) = %d, want 3 (same count as before, per 'preserva a data de quitação original')", len(created))
	}
	wantDates := []string{"2026-09-01", "2026-10-01", "2026-11-01"}
	sum := dec(t, "0")
	for i, ins := range created {
		if got := ins.ProjectedAt.Format("2006-01-02"); got != wantDates[i] {
			t.Errorf("installment[%d] date = %s, want %s", i, got, wantDates[i])
		}
		sum = sum.Add(ins.Amount)
	}
	if !sum.Equal(dec(t, "900.00")) {
		t.Errorf("sum = %s, want 900.00", sum)
	}
	if !created[0].Amount.Equal(dec(t, "300.00")) || !created[1].Amount.Equal(dec(t, "300.00")) || !created[2].Amount.Equal(dec(t, "300.00")) {
		t.Errorf("installments = %+v, want 300.00 each (900/3 divides evenly)", created)
	}
}

func TestReadjustPreservesInstallmentsWithAnyAllocation(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)

	paidPartially, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), nil)
	untouched1, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 2", dec(t, "400.00"), date(t, "2026-10-01"), nil)
	untouched2, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 3", dec(t, "400.00"), date(t, "2026-11-01"), nil)

	link := linkFixture(t, conn, d.ID, "-300.00")
	if _, err := scenarios.CreateRealization(ctx, conn, paidPartially.ID, link.ID, dec(t, "300.00")); err != nil {
		t.Fatalf("CreateRealization: %v", err)
	}

	// Debt still owes 900.00 total; 100.00 of that is "reserved" by the
	// partially-paid installment (400 - 300 = 100 still open on it), so
	// only 800.00 should be spread across the two untouched installments.
	created, err := scenarios.Readjust(ctx, conn, s.ID, dec(t, "900.00"), scenarios.StrategyRedistribute)
	if err != nil {
		t.Fatalf("Readjust: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("len(created) = %d, want 2 (only the two unallocated installments)", len(created))
	}
	sum := created[0].Amount.Add(created[1].Amount)
	if !sum.Equal(dec(t, "800.00")) {
		t.Errorf("sum of new installments = %s, want 800.00 (900 - 100 reserved)", sum)
	}

	// The partially-paid installment must survive untouched.
	stillThere, err := scenarios.GetScenarioTransaction(ctx, conn, paidPartially.ID)
	if err != nil {
		t.Fatalf("GetScenarioTransaction(paidPartially): %v", err)
	}
	if !stillThere.Amount.Equal(dec(t, "400.00")) {
		t.Errorf("preserved installment amount changed: %s", stillThere.Amount)
	}
	if _, err := scenarios.GetScenarioTransaction(ctx, conn, untouched1.ID); !errors.Is(err, scenarios.ErrTransactionNotFound) {
		t.Errorf("untouched1 should have been replaced: err = %v", err)
	}
	if _, err := scenarios.GetScenarioTransaction(ctx, conn, untouched2.ID); !errors.Is(err, scenarios.ErrTransactionNotFound) {
		t.Errorf("untouched2 should have been replaced: err = %v", err)
	}
}

func TestReadjustFailsWhenEverythingIsAllocated(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)
	st, _ := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), nil)
	link := linkFixture(t, conn, d.ID, "-400.00")
	if _, err := scenarios.CreateRealization(ctx, conn, st.ID, link.ID, dec(t, "400.00")); err != nil {
		t.Fatalf("CreateRealization: %v", err)
	}

	if _, err := scenarios.Readjust(ctx, conn, s.ID, dec(t, "0.00"), scenarios.StrategyRedistribute); !errors.Is(err, scenarios.ErrNoAffectedInstallments) {
		t.Errorf("Readjust: err = %v, want ErrNoAffectedInstallments", err)
	}
}

func TestReadjustFailsWhenNothingLeftToRedistribute(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)
	scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), nil)

	if _, err := scenarios.Readjust(ctx, conn, s.ID, dec(t, "0.00"), scenarios.StrategyRedistribute); !errors.Is(err, scenarios.ErrNothingToRedistribute) {
		t.Errorf("Readjust: err = %v, want ErrNothingToRedistribute", err)
	}
}

func TestReadjustRejectsInvalidStrategy(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)
	scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela 1", dec(t, "400.00"), date(t, "2026-09-01"), nil)

	if _, err := scenarios.Readjust(ctx, conn, s.ID, dec(t, "400.00"), scenarios.ReadjustStrategy("bogus")); !errors.Is(err, scenarios.ErrInvalidStrategy) {
		t.Errorf("Readjust: err = %v, want ErrInvalidStrategy", err)
	}
}
