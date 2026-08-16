package payables_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
	"contadinho-go/internal/payables"
)

func (f *fixture) addBill(accountID, externalID, dueDate string) {
	f.t.Helper()
	id := uuid.NewString()
	now := db.FormatTime(time.Now())
	due, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		f.t.Fatal(err)
	}
	f.exec(`INSERT INTO financial_bills (
			id, source_id, account_id, external_id, due_date,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 'hash', ?, ?)`,
		id, f.sourceID, accountID, externalID, db.FormatTime(due), f.rawImportID, now, now)
}

func TestProjectedEntryDateByBillID(t *testing.T) {
	f := newFixture(t)
	account := f.addAccount("BRL")
	f.addBill(account, "bill-1", "2026-08-10")
	f.addBill(account, "bill-2", "2026-09-10")

	dueDates, err := payables.FetchCardDueDates(context.Background(), f.conn)
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
	account := f.addAccount("BRL")
	f.addBill(account, "bill-1", "2026-08-10")
	f.addBill(account, "bill-2", "2026-09-10")

	dueDates, err := payables.FetchCardDueDates(context.Background(), f.conn)
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
	account := f.addAccount("BRL")
	f.addBill(account, "bill-1", "2026-08-10")

	dueDates, err := payables.FetchCardDueDates(context.Background(), f.conn)
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
	account := f.addAccount("BRL")
	f.addBill(account, "bill-1", "2026-08-31") // day 31, a short-month edge case

	dueDates, err := payables.FetchCardDueDates(context.Background(), f.conn)
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

func TestCreditAccountIDs(t *testing.T) {
	f := newFixture(t)
	checking := f.addAccount("BRL")
	card := f.addAccount("BRL")
	f.exec(`UPDATE financial_accounts SET account_type = 'CREDIT' WHERE id = ?`, card)

	ids, err := payables.CreditAccountIDs(context.Background(), f.conn)
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
