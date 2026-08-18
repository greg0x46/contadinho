package timeline_test

import (
	"context"
	"testing"

	"contadinho-go/internal/timeline"
)

func TestBuildSeriesStartingBalanceSumsAccounts(t *testing.T) {
	f := newFixture(t)
	f.addAccount("1000.00")
	f.addAccount("500.00")

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-01-01"), To: date(t, "2026-12-31"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if series.StartingBalance.String() != "1500" {
		t.Errorf("StartingBalance = %s, want 1500", series.StartingBalance.String())
	}
}

func TestBuildSeriesStartingBalanceFilteredByAccount(t *testing.T) {
	f := newFixture(t)
	a := f.addAccount("1000.00")
	f.addAccount("500.00")

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-01-01"), To: date(t, "2026-12-31"), ReferenceDate: date(t, "2026-08-15"),
		AccountIDs: []string{a},
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if series.StartingBalance.String() != "1000" {
		t.Errorf("StartingBalance = %s, want 1000", series.StartingBalance.String())
	}
}

func TestBuildSeriesStartingBalanceExcludesCreditCards(t *testing.T) {
	f := newFixture(t)
	f.addAccount("1000.00")
	creditCard := f.addAccount("500.00")
	f.exec(`UPDATE financial_accounts SET account_type = 'CREDIT' WHERE id = ?`, creditCard)

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-01-01"), To: date(t, "2026-12-31"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if series.StartingBalance.String() != "1000" {
		t.Errorf("StartingBalance = %s, want 1000 (credit card balance excluded)", series.StartingBalance.String())
	}
}

func TestBuildSeriesCreditCardEntryProjectsToBillDueDate(t *testing.T) {
	f := newFixture(t)
	card := f.addCreditCardAccount("0")
	f.addBill(card, "bill-1", "2026-09-10")
	cat := categorySupermercado
	metadata := `{"billId":"bill-1"}`
	f.addTransaction(txn{
		AccountID: card, Amount: "-150.00", OccurredAt: date(t, "2026-08-20"),
		CategoryID: &cat, CreditCardMetadata: &metadata,
	})

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-01-01"), To: date(t, "2026-12-31"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if len(series.Entries) != 1 {
		t.Fatalf("Entries = %d, want 1", len(series.Entries))
	}
	entry := series.Entries[0]
	if !entry.Date.Equal(date(t, "2026-09-10")) {
		t.Errorf("Date = %s, want 2026-09-10 (the bill's due date, not occurred_at)", entry.Date)
	}
	if entry.Tier != timeline.TierConfirmado {
		t.Errorf("Tier = %s, want Confirmado", entry.Tier)
	}
	if entry.Amount.String() != "-150" {
		t.Errorf("Amount = %s, want -150", entry.Amount.String())
	}
}

func TestBuildSeriesCreditCardEntryWithoutBillFallsBackToOccurredAt(t *testing.T) {
	f := newFixture(t)
	card := f.addCreditCardAccount("0")
	cat := categorySupermercado
	f.addTransaction(txn{
		AccountID: card, Amount: "-40.00", OccurredAt: date(t, "2026-08-20"), CategoryID: &cat,
	})

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-01-01"), To: date(t, "2026-12-31"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if len(series.Entries) != 1 {
		t.Fatalf("Entries = %d, want 1", len(series.Entries))
	}
	if !series.Entries[0].Date.Equal(date(t, "2026-08-20")) {
		t.Errorf("Date = %s, want 2026-08-20 (no bill history to project from)", series.Entries[0].Date)
	}
}

func TestBuildSeriesRealEntriesOnlyIncludesEligibleTransactions(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	cat := categorySupermercado
	f.addTransaction(txn{AccountID: account, Amount: "-100.00", OccurredAt: date(t, "2026-08-01"), CategoryID: &cat})
	sal := categorySalario
	f.addTransaction(txn{AccountID: account, Amount: "2000.00", OccurredAt: date(t, "2026-08-05"), CategoryID: &sal})

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-08-01"), To: date(t, "2026-08-31"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if len(series.Entries) != 2 {
		t.Fatalf("Entries = %+v, want 2", series.Entries)
	}
	for _, e := range series.Entries {
		if e.Tier != timeline.TierRealizado || e.Source != timeline.SourceReal {
			t.Errorf("entry = %+v, want TierRealizado/SourceReal", e)
		}
	}
}

func TestBuildSeriesEntriesFilteredByCategory(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	cat := categorySupermercado
	f.addTransaction(txn{AccountID: account, Amount: "-100.00", OccurredAt: date(t, "2026-08-01"), CategoryID: &cat})
	sal := categorySalario
	f.addTransaction(txn{AccountID: account, Amount: "2000.00", OccurredAt: date(t, "2026-08-05"), CategoryID: &sal})

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-08-01"), To: date(t, "2026-08-31"), ReferenceDate: date(t, "2026-08-15"),
		CategoryIDs: []string{categorySupermercado},
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if len(series.Entries) != 1 || series.Entries[0].CategoryName != "Supermercado" {
		t.Errorf("Entries = %+v, want only the Supermercado entry", series.Entries)
	}
}

// TestBuildSeriesBalanceAnchoredAtReference verifies the historical
// reconstruction described in build.go's buildPoints comment: with only
// past entries, every point's balance equals StartingBalance minus
// whatever hadn't happened yet at that point in time, so the balance at
// the most recent entry (and at every later day, M3 has no future
// sources) reads as exactly StartingBalance.
func TestBuildSeriesBalanceAnchoredAtReference(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("1000.00")
	f.addTransaction(txn{AccountID: account, Amount: "-100.00", OccurredAt: date(t, "2026-08-01")})
	f.addTransaction(txn{AccountID: account, Amount: "300.00", OccurredAt: date(t, "2026-08-10")})

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-08-01"), To: date(t, "2026-08-15"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	last := series.Points[len(series.Points)-1]
	if !last.Balance.Equal(series.StartingBalance) {
		t.Errorf("last point balance = %s, want %s (StartingBalance)", last.Balance, series.StartingBalance)
	}
	first := series.Points[0]
	// A DayPoint's balance is "as of the end of that day": on 2026-08-01,
	// only that day's own -100 entry has applied yet (the +300 on 08-10
	// hasn't happened), so its balance is StartingBalance minus what
	// hadn't happened yet as of end-of-day 08-01 (+300): 1000 - 300 = 700.
	if first.Balance.String() != "700" {
		t.Errorf("first point balance = %s, want 700", first.Balance.String())
	}
	if series.LowestBalance.Balance.String() != series.StartingBalance.String() {
		t.Errorf("LowestBalance = %s, want %s (M3 has no future sources, so it's flat at reference)",
			series.LowestBalance.Balance, series.StartingBalance)
	}
	if series.FirstNegative != nil {
		t.Errorf("FirstNegative = %v, want nil", series.FirstNegative)
	}
}

func TestBuildSeriesFirstNegativeWhenStartingBalanceIsAlreadyNegative(t *testing.T) {
	f := newFixture(t)
	f.addAccount("-50.00")

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-08-01"), To: date(t, "2026-08-31"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if series.FirstNegative == nil || !series.FirstNegative.Equal(date(t, "2026-08-15")) {
		t.Errorf("FirstNegative = %v, want 2026-08-15", series.FirstNegative)
	}
}
