package timeline_test

import (
	"context"
	"testing"

	"contadinho-go/internal/timeline"
)

// balanceOn returns the Balance of the DayPoint dated day, failing the test
// if no such point exists.
func balanceOn(t *testing.T, series timeline.Series, day string) string {
	t.Helper()
	target := date(t, day)
	for _, p := range series.Points {
		if p.Date.Equal(target) {
			return p.Balance.String()
		}
	}
	t.Fatalf("no DayPoint dated %s in series", day)
	return ""
}

// TestBuildSeriesTreatsCategorizedCardPaymentAsTransferNotDoubleCount is the
// whole point of internal/categories/cardpayment.go: once both legs of an
// early card-bill payment are categorized as the seeded transfer category,
// BuildSeries — unchanged — must not double-count the purchase's cash
// impact at the bill's due date.
//
// The mechanism needs no new code here because it already exists for any
// other transfer-kind category: the bank leg counts once, on the real
// payment date, because a transfer's MovesCash is true (money.MovedCash).
// The card leg sits on a CREDIT account, so realEntries re-dates it (via
// CardDueDates.ProjectedEntryDate) from its own occurred_at to the bill's
// due date — the same due date the original purchase was already re-dated
// to — where its positive amount exactly cancels the purchase's negative
// one. Net effect: the money leaves the modeled balance exactly once, on
// the day it actually left the bank.
func TestBuildSeriesTreatsCategorizedCardPaymentAsTransferNotDoubleCount(t *testing.T) {
	f := newFixture(t)
	bank := f.addAccount("1000.00")
	card := f.addCreditCardAccount("0")
	f.addBill(card, "bill-september", "2026-09-10")
	metadata := `{"billId":"bill-september"}`
	cardPaymentCategory := "dd10c680-fb35-4457-8595-4e51c8d279a7"

	// The purchase, on 2026-08-20 — re-dated by ProjectedEntryDate to the
	// due date regardless of any categorization here.
	f.addTransaction(txn{
		AccountID: card, Amount: "-150.00", OccurredAt: date(t, "2026-08-20"),
		CreditCardMetadata: &metadata,
	})
	// The bank leg: real money leaving the paying account on the actual
	// (early) payment date.
	f.addTransaction(txn{
		AccountID: bank, Amount: "-150.00", OccurredAt: date(t, "2026-09-02"),
		CategoryID: &cardPaymentCategory,
	})
	// The card leg: pays down the card, re-dated to the same due date as
	// the purchase because it carries the same billId.
	f.addTransaction(txn{
		AccountID: card, Amount: "150.00", OccurredAt: date(t, "2026-09-02"),
		CreditCardMetadata: &metadata, CategoryID: &cardPaymentCategory,
	})

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-08-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-09-03"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if len(series.Entries) != 3 {
		t.Fatalf("Entries = %+v, want all three legs present (nothing removed)", series.Entries)
	}
	if got := balanceOn(t, series, "2026-09-02"); got != "1000" {
		t.Errorf("balance right after payment = %s, want 1000 (real cash left once)", got)
	}
	if got := balanceOn(t, series, "2026-09-10"); got != "1000" {
		t.Errorf("balance at the former due date = %s, want 1000 (no second charge)", got)
	}
}

// TestBuildSeriesCardPaymentWithoutBillIDCancelsOnItsOwnDueDate reproduces
// a real production bug found in the wild: a "Pagamento recebido" carries
// no billId and a billForecastDate naming the *next* bill, not the one it
// actually pays. Before ProjectedEntryDate's isPaymentLeg branch, that
// combination misdated the payment a full cycle forward — decoupled from
// the purchase it was meant to cancel, showing as a large, unexplained
// balance jump on the *next* bill's due date instead of a wash on the
// correct one. This test has no billId on either leg (unlike
// TestBuildSeriesTreatsCategorizedCardPaymentAsTransferNotDoubleCount,
// which only proves the mechanism when Pluggy *does* supply one).
func TestBuildSeriesCardPaymentWithoutBillIDCancelsOnItsOwnDueDate(t *testing.T) {
	f := newFixture(t)
	bank := f.addAccount("1000.00")
	card := f.addCreditCardAccount("0")
	f.addBillWithClosing(card, "bill-august", "2026-08-02", "2026-08-10")
	cardPaymentCategory := "dd10c680-fb35-4457-8595-4e51c8d279a7"

	// The purchase: no billId, dated well before the August bill closed —
	// ProjectedEntryDate's ordinary cadence inference lands it on 08-10.
	f.addTransaction(txn{
		AccountID: card, Amount: "-150.00", OccurredAt: date(t, "2026-07-20"),
	})
	// The bank leg: real money leaving the paying account the day after
	// the bill closed — the ordinary prompt-payer case.
	f.addTransaction(txn{
		AccountID: bank, Amount: "-150.00", OccurredAt: date(t, "2026-08-03"),
		CategoryID: &cardPaymentCategory,
	})
	// The card leg: same day, no billId, and a billForecastDate pointing at
	// the *next* (not-yet-existing) bill — the exact shape observed in
	// production, source_category_id "05100000" included so
	// CardTransactionSignals recognizes it as a payment leg. isPaymentLeg
	// must still land this on 08-10, not a cycle forward.
	metadata := `{"billForecastDate":"2026-09"}`
	sourceCategoryID := "05100000"
	f.addTransaction(txn{
		AccountID: card, Amount: "150.00", OccurredAt: date(t, "2026-08-03"),
		CreditCardMetadata: &metadata, CategoryID: &cardPaymentCategory,
		SourceCategoryID: &sourceCategoryID,
	})

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-07-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-04"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if got := balanceOn(t, series, "2026-08-03"); got != "1000" {
		t.Errorf("balance right after payment = %s, want 1000 (real cash left once)", got)
	}
	if got := balanceOn(t, series, "2026-08-10"); got != "1000" {
		t.Errorf("balance at the bill's due date = %s, want 1000 (purchase and payment cancel here)", got)
	}
	if got := balanceOn(t, series, "2026-09-10"); got != "1000" {
		t.Errorf("balance a cycle later = %s, want 1000 (no misdated phantom jump)", got)
	}
}

// TestBuildSeriesPastBillCountsOnlyTheRealCashPayment covers the past half of
// the split cashEntries introduces. A bill that already fell due was paid for
// real, from a cash account, and that payment is in the series; the purchases
// re-dated onto the same due date are a forecast of a payment that has since
// happened, so walking them too charges the bill twice.
//
// The damage only shows when the two sides do not match, which is the normal
// case for any window that does not start at the account's birth: here the
// August bill settles a purchase from May, far outside even the fetch's
// reach-back, so the payment leg has no purchases to cancel against. Before
// the split the +300 card leg exactly offset the -300 bank leg in the anchor
// and the payment never appeared in the past curve at all — the same
// mechanism that put "Todo período" tens of thousands below reality.
func TestBuildSeriesPastBillCountsOnlyTheRealCashPayment(t *testing.T) {
	f := newFixture(t)
	bank := f.addAccount("1000.00")
	card := f.addCreditCardAccount("0")
	f.addBillWithClosing(card, "bill-august", "2026-08-02", "2026-08-10")
	cardPaymentCategory := "dd10c680-fb35-4457-8595-4e51c8d279a7"
	sourceCategoryID := "05100000"
	metadata := `{"billForecastDate":"2026-09"}`

	// The bank leg: 300 really left the checking account on the due date.
	f.addTransaction(txn{
		AccountID: bank, Amount: "-300.00", OccurredAt: date(t, "2026-08-10"),
		CategoryID: &cardPaymentCategory,
	})
	// The card leg, settling purchases made months before the window.
	f.addTransaction(txn{
		AccountID: card, Amount: "300.00", OccurredAt: date(t, "2026-08-10"),
		CategoryID: &cardPaymentCategory, SourceCategoryID: &sourceCategoryID,
		CreditCardMetadata: &metadata,
	})

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-08-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-09-07"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}
	if len(series.Entries) != 2 {
		t.Fatalf("Entries = %+v, want both legs still listed (only the walk changes)", series.Entries)
	}
	for day, want := range map[string]string{"2026-08-09": "1300", "2026-08-10": "1000", "2026-09-07": "1000"} {
		if got := balanceOn(t, series, day); got != want {
			t.Errorf("balance on %s = %s, want %s", day, got, want)
		}
	}
}

// TestTotalsForPeriodCountsASettledCardPurchaseThatPointsExcludes is the
// aggregate side of TestBuildSeriesTreatsCategorizedCardPaymentAsTransferNotDoubleCount:
// once the bill is paid, cashEntries drops the purchase from the balance
// walk so its cash is not charged twice (Points), but the money was still
// spent, and TotalsForPeriod — which walks the unfiltered series.Entries —
// must still count it as an expense.
func TestTotalsForPeriodCountsASettledCardPurchaseThatPointsExcludes(t *testing.T) {
	f := newFixture(t)
	bank := f.addAccount("1000.00")
	card := f.addCreditCardAccount("0")
	f.addBill(card, "bill-september", "2026-09-10")
	metadata := `{"billId":"bill-september"}`
	cat := categorySupermercado
	cardPaymentCategory := "dd10c680-fb35-4457-8595-4e51c8d279a7"

	f.addTransaction(txn{
		AccountID: card, Amount: "-150.00", OccurredAt: date(t, "2026-08-20"),
		CategoryID: &cat, CreditCardMetadata: &metadata,
	})
	f.addTransaction(txn{
		AccountID: bank, Amount: "-150.00", OccurredAt: date(t, "2026-09-02"),
		CategoryID: &cardPaymentCategory,
	})
	f.addTransaction(txn{
		AccountID: card, Amount: "150.00", OccurredAt: date(t, "2026-09-02"),
		CreditCardMetadata: &metadata, CategoryID: &cardPaymentCategory,
	})

	series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
		From: date(t, "2026-08-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-09-03"),
	})
	if err != nil {
		t.Fatalf("BuildSeries: %v", err)
	}

	totals := timeline.TotalsForPeriod(series)
	if got := totals.Expense.String(); got != "150" {
		t.Errorf("Expense = %s, want 150 (the purchase, even though Points dropped it as settled)", got)
	}
	if got := totals.Income.String(); got != "0" {
		t.Errorf("Income = %s, want 0 (both bill legs are transfer-categorized)", got)
	}
}

func TestBuildSeriesCardPaymentWithDelayedBill(t *testing.T) {
	for _, tc := range []struct{ name, payment, september, october string }{
		{"full payment", "150.00", "1000", "960"},
		{"partial payment", "100.00", "950", "910"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			bank := f.addAccount("1000.00")
			card := f.addCreditCardAccount("0")
			f.addBillWithClosing(card, "bill-august", "2026-08-02", "2026-08-10")
			category := "dd10c680-fb35-4457-8595-4e51c8d279a7"
			signal := "05100000"
			metadata := `{"billForecastDate":"2026-10"}`
			f.addTransaction(txn{AccountID: card, Amount: "-150.00", OccurredAt: date(t, "2026-08-20")})
			f.addTransaction(txn{AccountID: bank, Amount: "-" + tc.payment, OccurredAt: date(t, "2026-09-03"), CategoryID: &category})
			f.addTransaction(txn{AccountID: card, Amount: tc.payment, OccurredAt: date(t, "2026-09-03"), CategoryID: &category, SourceCategoryID: &signal, CreditCardMetadata: &metadata})
			f.addTransaction(txn{AccountID: card, Amount: "-40.00", OccurredAt: date(t, "2026-09-04")})
			for _, synced := range []bool{false, true} {
				if synced {
					f.addBillWithClosing(card, "bill-september", "2026-09-02", "2026-09-10")
				}
				series, err := timeline.BuildSeries(context.Background(), f.conn, timeline.BuildParams{
					From: date(t, "2026-09-01"), To: date(t, "2026-10-31"), ReferenceDate: date(t, "2026-09-07"),
				})
				if err != nil {
					t.Fatal(err)
				}
				for day, want := range map[string]string{"2026-09-07": "1000", "2026-09-10": tc.september, "2026-10-10": tc.october} {
					if got := balanceOn(t, series, day); got != want {
						t.Errorf("synced=%v balance on %s = %s, want %s", synced, day, got, want)
					}
				}
			}
		})
	}
}
