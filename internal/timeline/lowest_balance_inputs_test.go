package timeline_test

import (
	"context"
	"testing"

	"github.com/greg0x46/julius/internal/payables"
	"github.com/greg0x46/julius/internal/recurrences"
	"github.com/greg0x46/julius/internal/scenarios"
	"github.com/greg0x46/julius/internal/timeline"
)

// TestBuildSeriesLowestBalanceFromTodayCombinesEverySource pins the contract
// Home's daily allowance relies on: with From = ReferenceDate = today, the
// series' LowestBalance is the low point from today to month-end over
// recurring income and expenses, payable-plan installments and card bills
// re-dated to their due date, all together.
func TestBuildSeriesLowestBalanceFromTodayCombinesEverySource(t *testing.T) {
	f := newFixture(t)
	f.addAccount("3000.00")
	ctx := context.Background()

	if _, err := recurrences.Create(ctx, f.conn, recurrences.Write{
		Name: "Aluguel", Kind: recurrences.KindExpense, Amount: decT(t, "1500.00"),
		CategoryID: categorySupermercado,
		Cadence:    recurrences.CadenceMonthly, DayOfMonth: 12, StartDate: date(t, "2026-01-01"), IsActive: true,
	}); err != nil {
		t.Fatalf("recurrences.Create expense: %v", err)
	}
	if _, err := recurrences.Create(ctx, f.conn, recurrences.Write{
		Name: "Salário", Kind: recurrences.KindIncome, Amount: decT(t, "2000.00"),
		CategoryID: categorySalario,
		Cadence:    recurrences.CadenceMonthly, DayOfMonth: 20, StartDate: date(t, "2026-01-01"), IsActive: true,
	}); err != nil {
		t.Fatalf("recurrences.Create income: %v", err)
	}

	debt, err := payables.Create(ctx, f.conn, payables.KindDebt, "Financiamento", decT(t, "300.00"), decT(t, "300.00"))
	if err != nil {
		t.Fatalf("payables.Create: %v", err)
	}
	plan, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindDebtPlan, "Plano", &debt.ID)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, f.conn, plan.ID, "Parcela", decT(t, "300.00"), date(t, "2026-10-15"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}

	// Bought before today, paid on the bill's due date inside the window.
	card := f.addCreditCardAccount("0")
	f.addBill(card, "bill-oct", "2026-10-18")
	cat := categorySupermercado
	metadata := `{"billId":"bill-oct"}`
	f.addTransaction(txn{
		AccountID: card, Amount: "-400.00", OccurredAt: date(t, "2026-10-05"),
		CategoryID: &cat, CreditCardMetadata: &metadata,
	})

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-10-10"), To: date(t, "2026-10-31"), ReferenceDate: date(t, "2026-10-10"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}

	sources := map[timeline.SourceKind]int{}
	for _, e := range series.Entries {
		sources[e.Source]++
	}
	if sources[timeline.SourceRecurring] != 2 || sources[timeline.SourcePayablePlan] != 1 || sources[timeline.SourceReal] != 1 {
		t.Fatalf("entries by source = %v, want 2 recurring, 1 payable plan, 1 real (the card bill)", sources)
	}

	// 3000 on the 10th, -1500 rent on the 12th (1500), -300 installment on
	// the 15th (1200), -400 card bill on the 18th (800), +2000 salary on the
	// 20th (2800). Without the bill the low would be 1200 on the 15th.
	if !series.LowestBalance.Date.Equal(date(t, "2026-10-18")) || series.LowestBalance.Balance.String() != "800" {
		t.Errorf("LowestBalance = %s on %s, want 800 on 2026-10-18",
			series.LowestBalance.Balance, series.LowestBalance.Date.Format("2006-01-02"))
	}
	if final := series.Points[len(series.Points)-1]; final.Balance.String() != "2800" {
		t.Errorf("final balance = %s, want 2800", final.Balance)
	}
}
