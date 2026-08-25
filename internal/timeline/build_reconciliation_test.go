package timeline_test

import (
	"context"
	"testing"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/recurrences"
	"contadinho-go/internal/timeline"
)

// reconcilableCommitment builds the pairing the reconciliation tests need: a
// monthly expense plus the automation rule whose reconcile action targets it.
// Without that rule a commitment never reconciles at all, so every "the rule
// would have matched" assertion below needs both halves.
func reconcilableCommitment(t *testing.T, f *fixture) recurrences.RecurringCommitment {
	t.Helper()
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
	return commitment
}

func recurrenceEntriesOf(series timeline.Series) []timeline.Entry {
	var entries []timeline.Entry
	for _, e := range series.Entries {
		if e.Source == timeline.SourceRecurring {
			entries = append(entries, e)
		}
	}
	return entries
}

// This is what makes "desconciliar" mean anything: without the override
// reaching the Timeline the action would only relabel a row on screen while
// the projection stayed suppressed. September is reconciled by the rule in
// TestBuildSeriesReconciledRecurrenceEmitsNoEntry; detaching it must bring
// its projected entry — and its money — back.
func TestBuildSeriesDetachedOccurrenceProjectsAgain(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	ctx := context.Background()

	commitment := reconcilableCommitment(t, f)
	cat := categorySupermercado
	f.addTransaction(txn{AccountID: account, Amount: "-1490.00", OccurredAt: date(t, "2026-09-06"), CategoryID: &cat})

	if _, err := recurrences.PutOverride(ctx, f.conn, commitment.ID, date(t, "2026-09-05"),
		recurrences.StateDetached, nil); err != nil {
		t.Fatalf("PutOverride detach: %v", err)
	}

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-09-01"), To: date(t, "2026-10-31"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}

	entries := recurrenceEntriesOf(series)
	if len(entries) != 2 {
		t.Fatalf("recurrence entries = %+v, want September (detached, so projected again) and October", entries)
	}
	if !entries[0].Date.Equal(date(t, "2026-09-05")) || entries[0].Tier != timeline.TierProjetado {
		t.Errorf("first entry = %+v, want 2026-09-05 Projetado", entries[0])
	}

	// 1000 - 1490 (the real transaction) - 1500 (September, no longer
	// reconciled away) - 1500 (October).
	final := series.Points[len(series.Points)-1].Balance
	if final.String() != "-3490" {
		t.Errorf("final balance = %s, want -3490 (the detached occurrence back in the projection)", final)
	}
}

// A hand-picked transaction suppresses the occurrence exactly as a rule match
// does — the projection would otherwise double-count money the real
// transaction already carries.
func TestBuildSeriesManualLinkSuppressesTheOccurrence(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	ctx := context.Background()

	commitment := reconcilableCommitment(t, f)
	cat := categorySupermercado
	// Day 20 is outside the rule's 2:8 day window, so only a manual link can
	// reconcile October with it.
	transactionID := f.addTransaction(txn{AccountID: account, Amount: "-1450.00", OccurredAt: date(t, "2026-10-20"), CategoryID: &cat})

	if _, err := recurrences.PutOverride(ctx, f.conn, commitment.ID, date(t, "2026-10-05"),
		recurrences.StateLinked, &transactionID); err != nil {
		t.Fatalf("PutOverride link: %v", err)
	}

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-10-01"), To: date(t, "2026-10-31"), ReferenceDate: date(t, "2026-09-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}

	if entries := recurrenceEntriesOf(series); len(entries) != 0 {
		t.Fatalf("recurrence entries = %+v, want none (October was hand-linked)", entries)
	}
	// 1000 - 1450: the linked transaction counted once, no projection on top.
	final := series.Points[len(series.Points)-1].Balance
	if final.String() != "-450" {
		t.Errorf("final balance = %s, want -450", final)
	}
}

// Deleting the override is "voltar ao automático", not "desconciliar": the
// rule gets the occurrence back and suppresses it again.
func TestBuildSeriesDeletingTheOverrideRestoresTheRule(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	ctx := context.Background()

	commitment := reconcilableCommitment(t, f)
	cat := categorySupermercado
	f.addTransaction(txn{AccountID: account, Amount: "-1490.00", OccurredAt: date(t, "2026-09-06"), CategoryID: &cat})

	if _, err := recurrences.PutOverride(ctx, f.conn, commitment.ID, date(t, "2026-09-05"),
		recurrences.StateDetached, nil); err != nil {
		t.Fatalf("PutOverride detach: %v", err)
	}
	if err := recurrences.DeleteOverride(ctx, f.conn, commitment.ID, date(t, "2026-09-05")); err != nil {
		t.Fatalf("DeleteOverride: %v", err)
	}

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-09-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if entries := recurrenceEntriesOf(series); len(entries) != 0 {
		t.Fatalf("recurrence entries = %+v, want none (the rule reconciles September again)", entries)
	}
}

// The rule must not reuse a transaction another occurrence already claims by
// hand, or the same money would suppress two projections.
func TestBuildSeriesRuleSkipsAHandLinkedTransaction(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	ctx := context.Background()

	commitment := reconcilableCommitment(t, f)
	cat := categorySupermercado
	// Inside the rule's window for September, but hand-linked to October.
	transactionID := f.addTransaction(txn{AccountID: account, Amount: "-1490.00", OccurredAt: date(t, "2026-09-06"), CategoryID: &cat})

	if _, err := recurrences.PutOverride(ctx, f.conn, commitment.ID, date(t, "2026-10-05"),
		recurrences.StateLinked, &transactionID); err != nil {
		t.Fatalf("PutOverride link: %v", err)
	}

	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-09-01"), To: date(t, "2026-10-31"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}

	entries := recurrenceEntriesOf(series)
	if len(entries) != 1 || !entries[0].Date.Equal(date(t, "2026-09-05")) {
		t.Fatalf("recurrence entries = %+v, want only September's (October is hand-linked, and its transaction is off-limits to the rule)", entries)
	}
}

// The month-boundary case the widened override lookup exists for: a
// transaction hand-linked to February's occurrence lands in January, and the
// Timeline is asked only for January. If January's build didn't know about
// February's link, the rule would reuse that transaction and suppress
// January's projection — the wrong occurrence, settled by money that belongs
// to the next one.
func TestBuildSeriesSeesAManualLinkFromTheNeighbouringMonth(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	ctx := context.Background()

	commitment := reconcilableCommitment(t, f)
	cat := categorySupermercado
	// Inside the rule's 2:8 day window for September, hand-linked to October.
	transactionID := f.addTransaction(txn{AccountID: account, Amount: "-1490.00", OccurredAt: date(t, "2026-09-06"), CategoryID: &cat})
	if _, err := recurrences.PutOverride(ctx, f.conn, commitment.ID, date(t, "2026-10-05"),
		recurrences.StateLinked, &transactionID); err != nil {
		t.Fatalf("PutOverride link: %v", err)
	}

	// September alone — October's occurrence, and its override, fall outside.
	series, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-09-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}

	entries := recurrenceEntriesOf(series)
	if len(entries) != 1 || !entries[0].Date.Equal(date(t, "2026-09-05")) {
		t.Fatalf("recurrence entries = %+v, want September's own projection intact", entries)
	}
	// 1000 - 1490 (the real transaction) - 1500 (September, still projected).
	final := series.Points[len(series.Points)-1].Balance
	if final.String() != "-1990" {
		t.Errorf("final balance = %s, want -1990", final)
	}
}
