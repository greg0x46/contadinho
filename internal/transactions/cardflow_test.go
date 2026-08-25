package transactions_test

import (
	"context"
	"testing"
	"time"

	"contadinho-go/internal/transactions"
)

func TestProjectedEntryDateByBillID(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount(account{CurrencyCode: strp("BRL")})
	f.addBill(bill{AccountID: account, ExternalID: "bill-1", DueDate: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)})
	f.addBill(bill{AccountID: account, ExternalID: "bill-2", DueDate: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)})

	dueDates, err := transactions.FetchCardDueDates(context.Background(), f.conn)
	if err != nil {
		t.Fatalf("FetchCardDueDates: %v", err)
	}
	metadata := `{"billId":"bill-2"}`
	occurredAt, _ := time.Parse("2006-01-02", "2026-09-01")
	got := dueDates.ProjectedEntryDate(account, occurredAt, &metadata)
	want, _ := time.Parse("2006-01-02", "2026-09-10")
	if !got.Equal(want) {
		t.Errorf("ProjectedEntryDate = %s, want %s (bill-2's due date)", got, want)
	}
}

func TestProjectedEntryDateByForecastMonth(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount(account{CurrencyCode: strp("BRL")})
	f.addBill(bill{AccountID: account, ExternalID: "bill-1", DueDate: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)})
	f.addBill(bill{AccountID: account, ExternalID: "bill-2", DueDate: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)})

	dueDates, err := transactions.FetchCardDueDates(context.Background(), f.conn)
	if err != nil {
		t.Fatalf("FetchCardDueDates: %v", err)
	}
	// No billId (installment not yet billed), just Pluggy's forecast month.
	metadata := `{"billForecastDate":"2026-09"}`
	occurredAt, _ := time.Parse("2006-01-02", "2026-08-25")
	got := dueDates.ProjectedEntryDate(account, occurredAt, &metadata)
	want, _ := time.Parse("2006-01-02", "2026-09-10")
	if !got.Equal(want) {
		t.Errorf("ProjectedEntryDate = %s, want %s (bill matching forecast month)", got, want)
	}
}

func TestProjectedEntryDateInfersCadenceBeyondKnownBills(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount(account{CurrencyCode: strp("BRL")})
	f.addBill(bill{AccountID: account, ExternalID: "bill-1", DueDate: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)})

	dueDates, err := transactions.FetchCardDueDates(context.Background(), f.conn)
	if err != nil {
		t.Fatalf("FetchCardDueDates: %v", err)
	}
	// Still-open cycle, forecast month with no bill yet: infer forward from
	// the known due date's day-of-month (10th).
	metadata := `{"billForecastDate":"2026-10"}`
	occurredAt, _ := time.Parse("2006-01-02", "2026-09-25")
	got := dueDates.ProjectedEntryDate(account, occurredAt, &metadata)
	want, _ := time.Parse("2006-01-02", "2026-10-10")
	if !got.Equal(want) {
		t.Errorf("ProjectedEntryDate = %s, want %s (inferred cadence)", got, want)
	}
}

func TestProjectedEntryDateNoMetadataFallsBackToNextInferredDueDate(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount(account{CurrencyCode: strp("BRL")})
	f.addBill(bill{AccountID: account, ExternalID: "bill-1", DueDate: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}) // day 31, a short-month edge case

	dueDates, err := transactions.FetchCardDueDates(context.Background(), f.conn)
	if err != nil {
		t.Fatalf("FetchCardDueDates: %v", err)
	}
	// No credit_card_metadata at all: the open cycle after the last known
	// bill, clamped to February's actual length.
	occurredAt, _ := time.Parse("2006-01-02", "2027-02-15")
	got := dueDates.ProjectedEntryDate(account, occurredAt, nil)
	want, _ := time.Parse("2006-01-02", "2027-02-28")
	if !got.Equal(want) {
		t.Errorf("ProjectedEntryDate = %s, want %s (clamped to February)", got, want)
	}
}

// The days between a bill closing and falling due are the trap: the bill is
// still the nearest due date on the calendar, but it no longer accepts
// purchases, so dating a purchase there projects cash leaving on a bill
// that already closed — usually a date in the past, which drops the cost
// out of any forward-looking projection entirely.
func TestProjectedEntryDateSkipsBillAlreadyClosed(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount(account{CurrencyCode: strp("BRL")})
	f.addBill(bill{
		AccountID: account, ExternalID: "bill-1",
		ClosingDate: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
	})

	dueDates, err := transactions.FetchCardDueDates(context.Background(), f.conn)
	if err != nil {
		t.Fatalf("FetchCardDueDates: %v", err)
	}
	// Bought on the 9th: the bill due on the 10th closed a week earlier, so
	// this lands on the next cycle (inferred: closes 09-02, due 09-10).
	occurredAt, _ := time.Parse("2006-01-02", "2026-08-09")
	got := dueDates.ProjectedEntryDate(account, occurredAt, nil)
	want, _ := time.Parse("2006-01-02", "2026-09-10")
	if !got.Equal(want) {
		t.Errorf("ProjectedEntryDate = %s, want %s (next cycle, the open one)", got, want)
	}
}

func TestProjectedEntryDateUsesBillStillOpenAtPurchase(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount(account{CurrencyCode: strp("BRL")})
	f.addBill(bill{
		AccountID: account, ExternalID: "bill-1",
		ClosingDate: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
	})

	dueDates, err := transactions.FetchCardDueDates(context.Background(), f.conn)
	if err != nil {
		t.Fatalf("FetchCardDueDates: %v", err)
	}
	occurredAt, _ := time.Parse("2006-01-02", "2026-07-28")
	got := dueDates.ProjectedEntryDate(account, occurredAt, nil)
	want, _ := time.Parse("2006-01-02", "2026-08-10")
	if !got.Equal(want) {
		t.Errorf("ProjectedEntryDate = %s, want %s (the cycle open on the purchase day)", got, want)
	}
}

func TestCreditAccountIDs(t *testing.T) {
	f := newFixture(t)
	checking := f.addAccount(account{CurrencyCode: strp("BRL")})
	card := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")})

	ids, err := transactions.CreditAccountIDs(context.Background(), f.conn)
	if err != nil {
		t.Fatalf("CreditAccountIDs: %v", err)
	}
	if !ids[card] {
		t.Errorf("expected %s (CREDIT) to be included", card)
	}
	if ids[checking] {
		t.Errorf("expected %s (no account_type) to be excluded", checking)
	}
}
