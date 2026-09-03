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
