package timeline_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/payables"
	"contadinho-go/internal/recurrences"
	"contadinho-go/internal/rules"
	"contadinho-go/internal/scenarios"
	"contadinho-go/internal/timeline"
)

var testReconciliationConditions = []rules.Condition{
	{Field: rules.FieldAmount, Operator: rules.OperatorWithinPercent, Value: "1500.00:10"},
	{Field: rules.FieldDayOfMonth, Operator: rules.OperatorDayRange, Value: "2:8"},
}

func decT(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestBuildSeriesPayablePlanEntriesSignByPayableKind(t *testing.T) {
	f := newFixture(t)
	f.addAccount("1000.00")
	ctx := context.Background()

	debt, err := payables.Create(ctx, f.conn, payables.KindDebt, "Financiamento", decT(t, "500.00"), decT(t, "500.00"))
	if err != nil {
		t.Fatalf("payables.Create debt: %v", err)
	}
	debtScenario, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindDebtPlan, "Plano", &debt.ID)
	if err != nil {
		t.Fatalf("CreateScenario debt_plan: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, f.conn, debtScenario.ID, "Parcela 1", decT(t, "100.00"), date(t, "2026-09-01"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}

	receivable, err := payables.Create(ctx, f.conn, payables.KindReceivable, "Empréstimo dado", decT(t, "300.00"), decT(t, "300.00"))
	if err != nil {
		t.Fatalf("payables.Create receivable: %v", err)
	}
	receivableScenario, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindReceivablePlan, "Plano", &receivable.ID)
	if err != nil {
		t.Fatalf("CreateScenario receivable_plan: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, f.conn, receivableScenario.ID, "Recebimento 1", decT(t, "150.00"), date(t, "2026-09-10"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-08-15"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}

	var debtEntry, receivableEntry *timeline.Entry
	for i := range series.Entries {
		e := &series.Entries[i]
		if e.Source != timeline.SourcePayablePlan {
			continue
		}
		switch e.Description {
		case "Parcela 1":
			debtEntry = e
		case "Recebimento 1":
			receivableEntry = e
		}
	}
	if debtEntry == nil || debtEntry.Amount.String() != "-100" || debtEntry.Tier != timeline.TierConfirmado {
		t.Errorf("debt installment entry = %+v, want -100 Confirmado", debtEntry)
	}
	if receivableEntry == nil || receivableEntry.Amount.String() != "150" || receivableEntry.Tier != timeline.TierConfirmado {
		t.Errorf("receivable installment entry = %+v, want 150 Confirmado", receivableEntry)
	}
}

func TestBuildSeriesPayablePlanEntriesExcludesRealizedInstallments(t *testing.T) {
	f := newFixture(t)
	f.addAccount("1000.00")
	ctx := context.Background()

	debt, err := payables.Create(ctx, f.conn, payables.KindDebt, "Financiamento", decT(t, "500.00"), decT(t, "500.00"))
	if err != nil {
		t.Fatalf("payables.Create: %v", err)
	}
	plan, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindDebtPlan, "Plano", &debt.ID)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	installment, err := scenarios.CreateScenarioTransaction(ctx, f.conn, plan.ID, "Parcela 1", decT(t, "100.00"), date(t, "2026-09-01"), nil)
	if err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}
	// Realize it via a real payable link, mirroring the HTTP flow.
	account := f.addAccount("0.00")
	txID := f.addTransaction(txn{AccountID: account, Amount: "-100.00", OccurredAt: date(t, "2026-09-01")})
	linkResult, err := payables.CreateLink(ctx, f.conn, payables.KindDebt, debt.ID, txID)
	if err != nil {
		t.Fatalf("payables.CreateLink: %v", err)
	}
	if linkResult.Link == nil {
		t.Fatalf("payables.CreateLink() = %+v, want a Link", linkResult)
	}
	if _, err := scenarios.CreateRealization(ctx, f.conn, installment.ID, &linkResult.Link.ID, decT(t, "100.00")); err != nil {
		t.Fatalf("CreateRealization: %v", err)
	}

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-08-15"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	for _, e := range series.Entries {
		if e.Source == timeline.SourcePayablePlan {
			t.Errorf("realized installment must not appear as a payable-plan entry, got %+v", e)
		}
	}
}

func TestBuildSeriesRecurrenceEntriesUseExpectedAmountWithoutMatch(t *testing.T) {
	f := newFixture(t)
	f.addAccount("1000.00")
	ctx := context.Background()

	commitment, err := recurrences.Create(ctx, f.conn, recurrences.Write{
		Name: "Aluguel", Kind: recurrences.KindExpense, Amount: decT(t, "1500.00"),
		CategoryID: categorySupermercado,
		Cadence:    recurrences.CadenceMonthly, DayOfMonth: 5, StartDate: date(t, "2026-01-01"), IsActive: true,
	})
	if err != nil {
		t.Fatalf("recurrences.Create: %v", err)
	}
	_ = commitment

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-09-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	var recurrenceEntry *timeline.Entry
	for i := range series.Entries {
		if series.Entries[i].Source == timeline.SourceRecurring {
			recurrenceEntry = &series.Entries[i]
		}
	}
	// Projetado, not Confirmado: the occurrence is scheduled and carries
	// its expected amount, but nothing real has been observed for it.
	if recurrenceEntry == nil || recurrenceEntry.Amount.String() != "-1500" || recurrenceEntry.Tier != timeline.TierProjetado {
		t.Fatalf("recurrence entry = %+v, want -1500 Projetado", recurrenceEntry)
	}
	if !recurrenceEntry.Date.Equal(date(t, "2026-09-05")) {
		t.Errorf("recurrence entry date = %v, want 2026-09-05", recurrenceEntry.Date)
	}
}

// TestBuildSeriesReconciledRecurrenceEmitsNoEntry pins the rule that keeps
// a reconciled commitment from being counted twice: the real transaction is
// already in the series via realEntries, so its occurrence must not also
// appear. September's occurrence is reconciled and drops out; October's has
// no transaction of its own and stays, at its expected amount rather than
// September's actual one — the candidates a month's occurrence resolves
// against are scoped to that month.
func TestBuildSeriesReconciledRecurrenceEmitsNoEntry(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	ctx := context.Background()

	commitment, err := recurrences.Create(ctx, f.conn, recurrences.Write{
		Name: "Aluguel", Kind: recurrences.KindExpense, Amount: decT(t, "1500.00"),
		CategoryID: categorySupermercado,
		Cadence:    recurrences.CadenceMonthly, DayOfMonth: 5, StartDate: date(t, "2026-01-01"), IsActive: true,
	})
	if err != nil {
		t.Fatalf("recurrences.Create: %v", err)
	}
	if _, err := automation.Create(ctx, f.conn, automation.Write{
		Name: "Concilia aluguel", IsActive: true, LogicOperator: automation.LogicAnd,
		Conditions: testReconciliationConditions,
		Actions:    []automation.ActionWrite{{Type: automation.ActionReconcile, ScenarioID: &commitment.ID}},
	}); err != nil {
		t.Fatalf("automation.Create: %v", err)
	}
	cat := categorySupermercado
	f.addTransaction(txn{AccountID: account, Amount: "-1490.00", OccurredAt: date(t, "2026-09-06"), CategoryID: &cat})

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-09-01"), To: date(t, "2026-10-31"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}

	var recurrenceEntries []timeline.Entry
	for _, e := range series.Entries {
		if e.Source == timeline.SourceRecurring {
			recurrenceEntries = append(recurrenceEntries, e)
		}
	}
	if len(recurrenceEntries) != 1 {
		t.Fatalf("recurrence entries = %+v, want only October's (September's was reconciled away)", recurrenceEntries)
	}
	october := recurrenceEntries[0]
	if !october.Date.Equal(date(t, "2026-10-05")) {
		t.Errorf("recurrence entry date = %v, want 2026-10-05", october.Date)
	}
	if october.Amount.String() != "-1500" || october.Tier != timeline.TierProjetado {
		t.Errorf("october entry = %+v, want -1500 Projetado (its own expected amount, not September's -1490)", october)
	}

	// The regression this test exists for: 1000 - 1490 (real, counted once)
	// - 1500 (October, projected).
	final := series.Points[len(series.Points)-1].Balance
	if final.String() != "-1990" {
		t.Errorf("final balance = %s, want -1990 (the reconciled amount counted exactly once)", final)
	}
}

// TestBuildSeriesReconciliationScopeIsYearAware pins the year half of the
// candidate scope. A window longer than twelve months contains the same
// calendar month twice; September 2026's real transaction must reconcile
// only September 2026's occurrence, never September 2027's.
func TestBuildSeriesReconciliationScopeIsYearAware(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	ctx := context.Background()

	commitment, err := recurrences.Create(ctx, f.conn, recurrences.Write{
		Name: "Aluguel", Kind: recurrences.KindExpense, Amount: decT(t, "1500.00"),
		CategoryID: categorySupermercado,
		Cadence:    recurrences.CadenceMonthly, DayOfMonth: 5, StartDate: date(t, "2026-01-01"), IsActive: true,
	})
	if err != nil {
		t.Fatalf("recurrences.Create: %v", err)
	}
	if _, err := automation.Create(ctx, f.conn, automation.Write{
		Name: "Concilia aluguel", IsActive: true, LogicOperator: automation.LogicAnd,
		Conditions: testReconciliationConditions,
		Actions:    []automation.ActionWrite{{Type: automation.ActionReconcile, ScenarioID: &commitment.ID}},
	}); err != nil {
		t.Fatalf("automation.Create: %v", err)
	}
	cat := categorySupermercado
	f.addTransaction(txn{AccountID: account, Amount: "-1490.00", OccurredAt: date(t, "2026-09-06"), CategoryID: &cat})

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-09-01"), To: date(t, "2027-09-30"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}

	var septembers []timeline.Entry
	for _, e := range series.Entries {
		if e.Source == timeline.SourceRecurring && e.Date.Month() == time.September {
			septembers = append(septembers, e)
		}
	}
	if len(septembers) != 1 {
		t.Fatalf("September recurrence entries = %+v, want only 2027's (2026's was reconciled away)", septembers)
	}
	if !septembers[0].Date.Equal(date(t, "2027-09-05")) {
		t.Errorf("surviving September entry = %v, want 2027-09-05 — 2026's transaction must not reconcile 2027", septembers[0].Date)
	}
}

func TestBuildSeriesRecurrenceEntriesNeverReconciledWithoutLinkedRule(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	ctx := context.Background()

	if _, err := recurrences.Create(ctx, f.conn, recurrences.Write{
		Name: "Aluguel", Kind: recurrences.KindExpense, Amount: decT(t, "1500.00"),
		CategoryID: categorySupermercado,
		Cadence:    recurrences.CadenceMonthly, DayOfMonth: 5, StartDate: date(t, "2026-01-01"), IsActive: true,
	}); err != nil {
		t.Fatalf("recurrences.Create: %v", err)
	}
	// A real transaction that would satisfy testReconciliationConditions
	// (were it evaluated) is present, but no automation rule links to this
	// commitment — it must never be treated as matched. Its amount (-1490)
	// deliberately differs from the expected amount (-1500) so a wrongly
	// "matched" result would be distinguishable from the correct one.
	cat := categorySupermercado
	f.addTransaction(txn{AccountID: account, Amount: "-1490.00", OccurredAt: date(t, "2026-09-05"), CategoryID: &cat})

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-09-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	var recurrenceEntry *timeline.Entry
	for i := range series.Entries {
		if series.Entries[i].Source == timeline.SourceRecurring {
			recurrenceEntry = &series.Entries[i]
		}
	}
	if recurrenceEntry == nil || recurrenceEntry.Amount.String() != "-1500" {
		t.Fatalf("recurrence entry = %+v, want the expected amount -1500 (no linked rule to reconcile against)", recurrenceEntry)
	}
}

// TestBuildSeriesLowestBalanceAndFirstNegative is the M4 fixture the spec
// asks for: a designed dip below zero on a specific future date.
func TestBuildSeriesLowestBalanceAndFirstNegative(t *testing.T) {
	f := newFixture(t)
	f.addAccount("1000.00")
	ctx := context.Background()

	debt, err := payables.Create(ctx, f.conn, payables.KindDebt, "Cartão", decT(t, "2000.00"), decT(t, "2000.00"))
	if err != nil {
		t.Fatalf("payables.Create: %v", err)
	}
	plan, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindDebtPlan, "Plano", &debt.ID)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, f.conn, plan.ID, "Parcela grande", decT(t, "1200.00"), date(t, "2026-09-01"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-08-15"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}

	if !series.LowestBalance.Date.Equal(date(t, "2026-09-01")) || series.LowestBalance.Balance.String() != "-200" {
		t.Errorf("LowestBalance = %+v, want -200 on 2026-09-01", series.LowestBalance)
	}
	if series.FirstNegative == nil || !series.FirstNegative.Equal(date(t, "2026-09-01")) {
		t.Errorf("FirstNegative = %v, want 2026-09-01", series.FirstNegative)
	}
}

// TestBuildSeriesProjectsForwardToAFutureTo mirrors the real API usage: the
// frontend always passes today as ReferenceDate (it's the point
// StartingBalance is true as of) and a future To to see the projected
// balance further out — an installment between today and To must still
// show up in the projection, added onward from StartingBalance.
func TestBuildSeriesProjectsForwardToAFutureTo(t *testing.T) {
	f := newFixture(t)
	f.addAccount("1000.00")
	ctx := context.Background()

	debt, err := payables.Create(ctx, f.conn, payables.KindDebt, "Financiamento", decT(t, "500.00"), decT(t, "500.00"))
	if err != nil {
		t.Fatalf("payables.Create: %v", err)
	}
	plan, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindDebtPlan, "Plano", &debt.ID)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, f.conn, plan.ID, "Parcela 1", decT(t, "100.00"), date(t, "2026-09-15"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-08-15"), To: date(t, "2026-12-31"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	last := series.Points[len(series.Points)-1]
	if last.Balance.String() != "900" {
		t.Errorf("projected balance at end of range = %s, want 900 (1000 - 100 installment)", last.Balance.String())
	}
}
