package transactions_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/transactions"
)

// TestCreditCardTotalIgnoresTransferCategory is the guard rail on the other
// side of the transfer rule. Paying a card bill IS a transfer between the
// user's own accounts, so it is exactly the kind of transaction a user would
// categorize as one. But the card total answers "how much will this bill
// charge me", and a payment that stops being subtracted inflates the debt.
// The transfer rule must not reach this calculation.
func TestCreditCardTotalIgnoresTransferCategory(t *testing.T) {
	f := newFixture(t)
	loc := time.UTC
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, loc)
	lastClosing := time.Date(2026, 6, 2, 0, 0, 0, 0, loc)
	purchasedAt := time.Date(2026, 6, 5, 12, 0, 0, 0, loc)

	card := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")})
	f.addBill(bill{AccountID: card, ExternalID: "bill-history", DueDate: lastClosing, ClosingDate: lastClosing})

	f.addTransaction(txn{
		AccountID: card, Description: strp("Compra"), Amount: strp("-200.00"),
		AmountInAccountCurrency: strp("-200.00"), CurrencyCode: strp("BRL"),
		OccurredAt: &purchasedAt, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT"),
	})

	payment := f.addTransaction(txn{
		AccountID: card, Description: strp("Pagamento de fatura"), Amount: strp("50.00"),
		AmountInAccountCurrency: strp("50.00"), CurrencyCode: strp("BRL"),
		OccurredAt: &purchasedAt, ProviderStatus: strp("POSTED"), MovementType: strp("CREDIT"),
	})
	f.setCategory(payment, categoryTransferencia)

	total, err := transactions.CreditCardTransactionTotalAt(context.Background(), f.conn, "BRL", now, loc)
	if err != nil {
		t.Fatalf("CreditCardTransactionTotalAt: %v", err)
	}
	// 200 owed minus the 50 payment. If the transfer rule leaked in, the
	// payment would drop out and this would read 200.
	if total.String() != "150" {
		t.Errorf("total = %s, want 150 — the transfer-categorized payment must still be subtracted", total)
	}
}

// TestCreditCardTotalExcludesBillPaymentFromWrongCycle reproduces a real
// production bug: a bill payment normally lands (and is paid) in the 7-10
// day gap between a bill's closing_date and its due_date — a window that,
// by currentCreditCardCycle's own definition, already belongs to the *next*
// still-open cycle. Pluggy's "Pagamento recebido" credit typically carries
// no billId, so before the fix it fell back to date-based classification
// and was misattributed as debt reduction on a cycle it has nothing to do
// with, driving the total negative the moment the payment was no longer
// excluded via an ignored inclusion decision (see
// categories.IsCardPaymentTransaction / cardTransactionBelongsToCurrentCycle).
func TestCreditCardTotalExcludesBillPaymentFromWrongCycle(t *testing.T) {
	f := newFixture(t)
	loc := time.UTC
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, loc)
	julyClosing := time.Date(2026, 7, 2, 0, 0, 0, 0, loc)
	augustClosing := time.Date(2026, 8, 2, 0, 0, 0, 0, loc)
	augustDue := time.Date(2026, 8, 10, 0, 0, 0, 0, loc)
	newPurchaseAt := time.Date(2026, 8, 5, 12, 0, 0, 0, loc)
	paymentAt := time.Date(2026, 8, 11, 9, 0, 0, 0, loc) // right after the August bill's due date

	card := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")})
	f.addBill(bill{AccountID: card, ExternalID: "bill-july", DueDate: time.Date(2026, 7, 9, 0, 0, 0, 0, loc), ClosingDate: julyClosing})
	f.addBill(bill{AccountID: card, ExternalID: "bill-august", DueDate: augustDue, ClosingDate: augustClosing})

	// A genuine new purchase, already on the currently open (not yet billed) cycle.
	f.addTransaction(txn{
		AccountID: card, Description: strp("Compra"), Amount: strp("-50.00"),
		AmountInAccountCurrency: strp("-50.00"), CurrencyCode: strp("BRL"),
		OccurredAt: &newPurchaseAt, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT"),
	})

	// The August bill's payment: real Pluggy shape — no billId in
	// credit_card_metadata, recognized instead by source_category_id.
	f.addTransaction(txn{
		AccountID: card, Description: strp("Pagamento recebido"), Amount: strp("200.00"),
		AmountInAccountCurrency: strp("200.00"), CurrencyCode: strp("BRL"),
		OccurredAt: &paymentAt, ProviderStatus: strp("POSTED"), MovementType: strp("CREDIT"),
		SourceCategoryID: strp("05100000"),
	})

	total, err := transactions.CreditCardTransactionTotalAt(context.Background(), f.conn, "BRL", now, loc)
	if err != nil {
		t.Fatalf("CreditCardTransactionTotalAt: %v", err)
	}
	// Only the new purchase belongs to the open cycle; the payment settles
	// the already-closed August bill and must not touch this total at all.
	if total.String() != "50" {
		t.Errorf("total = %s, want 50 — the bill payment must not be attributed to the new cycle", total)
	}
}

// TestCreditCardTotalCountsPaymentAgainstTheStillOpenBill covers the one
// resolved-billId case cardTransactionBelongsToCurrentCycle's doc comment
// calls out explicitly: a payment leg whose billId names the currently
// open (not yet closed) bill — an early/partial payment — must still reduce
// the cycle total, unlike a payment with no resolvable billId at all (see
// TestCreditCardTotalExcludesBillPaymentFromWrongCycle), which is excluded
// because occurred_at alone is not a safe signal.
func TestCreditCardTotalCountsPaymentAgainstTheStillOpenBill(t *testing.T) {
	f := newFixture(t)
	loc := time.UTC
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, loc)
	augustClosing := time.Date(2026, 8, 2, 0, 0, 0, 0, loc)
	septemberClosing := time.Date(2026, 9, 2, 0, 0, 0, 0, loc) // still open at `now`
	purchaseAt := time.Date(2026, 8, 5, 12, 0, 0, 0, loc)
	earlyPaymentAt := time.Date(2026, 8, 20, 9, 0, 0, 0, loc)

	card := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")})
	f.addBill(bill{AccountID: card, ExternalID: "bill-august", DueDate: time.Date(2026, 8, 10, 0, 0, 0, 0, loc), ClosingDate: augustClosing})
	f.addBill(bill{AccountID: card, ExternalID: "bill-september", DueDate: time.Date(2026, 9, 10, 0, 0, 0, 0, loc), ClosingDate: septemberClosing})

	f.addTransaction(txn{
		AccountID: card, Description: strp("Compra"), Amount: strp("-300.00"),
		AmountInAccountCurrency: strp("-300.00"), CurrencyCode: strp("BRL"),
		OccurredAt: &purchaseAt, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT"),
	})

	f.addTransaction(txn{
		AccountID: card, Description: strp("Pagamento antecipado"), Amount: strp("100.00"),
		AmountInAccountCurrency: strp("100.00"), CurrencyCode: strp("BRL"),
		OccurredAt: &earlyPaymentAt, ProviderStatus: strp("POSTED"), MovementType: strp("CREDIT"),
		SourceCategoryID:   strp("05100000"),
		CreditCardMetadata: strp(`{"billId":"bill-september"}`),
	})

	total, err := transactions.CreditCardTransactionTotalAt(context.Background(), f.conn, "BRL", now, loc)
	if err != nil {
		t.Fatalf("CreditCardTransactionTotalAt: %v", err)
	}
	if total.String() != "200" {
		t.Errorf("total = %s, want 200 — an early payment against the open bill must reduce what it owes", total)
	}
}

func TestCreditCardTotalExcludesDeletedManualTransaction(t *testing.T) {
	f := newFixture(t)
	loc := time.UTC
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, loc)
	lastClosing := time.Date(2026, 6, 2, 0, 0, 0, 0, loc)
	card := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")})
	f.addBill(bill{AccountID: card, ExternalID: "bill-history", DueDate: lastClosing, ClosingDate: lastClosing})

	id, err := transactions.CreateManual(context.Background(), f.conn, transactions.ManualInput{
		AccountID: card, Description: "Compra manual", Amount: decimal.RequireFromString("-200"), OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	if err := transactions.DeleteManual(context.Background(), f.conn, id, nil); err != nil {
		t.Fatalf("DeleteManual: %v", err)
	}

	total, err := transactions.CreditCardTransactionTotalAt(context.Background(), f.conn, "BRL", now, loc)
	if err != nil {
		t.Fatalf("CreditCardTransactionTotalAt: %v", err)
	}
	if !total.IsZero() {
		t.Errorf("total = %s, want 0 after deleting the manual transaction", total)
	}
}
