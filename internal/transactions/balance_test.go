package transactions_test

import (
	"context"
	"testing"

	"contadinho-go/internal/transactions"
)

// TestCashOnHandExcludesCreditAccounts covers the reason this figure is not
// just "sum every balance": a credit card's balance is debt owed, counted on
// the liability side by CreditCardTransactionTotal instead. An account whose
// provider never reported an account_type is cash, not credit.
func TestCashOnHandExcludesCreditAccounts(t *testing.T) {
	f := newFixture(t)
	f.addAccount(account{AccountType: strp("BANK"), Balance: strp("1500.00")})
	f.addAccount(account{AccountType: nil, Balance: strp("250.50")})
	f.addAccount(account{AccountType: strp("CREDIT"), Balance: strp("-900.00")})

	total, err := transactions.CashOnHand(context.Background(), f.conn, nil)
	if err != nil {
		t.Fatalf("CashOnHand: %v", err)
	}
	if total.String() != "1750.5" {
		t.Errorf("CashOnHand = %s, want 1750.5 (the CREDIT account's balance is debt, not cash)", total)
	}
}

// TestCashOnHandScopedToAccountIDsFiltersUntypedAccounts guards the SQL
// precedence trap the query's comment calls out: without the parentheses
// around the account_type OR, an account with a NULL account_type binds to
// the wrong side of the AND and escapes the id filter entirely.
func TestCashOnHandScopedToAccountIDsFiltersUntypedAccounts(t *testing.T) {
	f := newFixture(t)
	wanted := f.addAccount(account{AccountType: strp("BANK"), Balance: strp("100.00")})
	f.addAccount(account{AccountType: nil, Balance: strp("9999.00")})

	total, err := transactions.CashOnHand(context.Background(), f.conn, []string{wanted})
	if err != nil {
		t.Fatalf("CashOnHand: %v", err)
	}
	if total.String() != "100" {
		t.Errorf("CashOnHand = %s, want 100 — the untyped account is outside the id filter", total)
	}
}

// TestCashOnHandIgnoresAccountsWithoutBalance pins that an account the
// provider has not reported a balance for contributes nothing, rather than
// failing the whole sum on an unparseable empty string.
func TestCashOnHandIgnoresAccountsWithoutBalance(t *testing.T) {
	f := newFixture(t)
	f.addAccount(account{AccountType: strp("BANK"), Balance: strp("42.00")})
	f.addAccount(account{AccountType: strp("BANK"), Balance: nil})

	total, err := transactions.CashOnHand(context.Background(), f.conn, nil)
	if err != nil {
		t.Fatalf("CashOnHand: %v", err)
	}
	if total.String() != "42" {
		t.Errorf("CashOnHand = %s, want 42", total)
	}
}

// TestCashOnHandIsZeroWithNoAccounts pins the empty case: no rows is zero,
// not an error — the timeline anchors on this before any sync has run.
func TestCashOnHandIsZeroWithNoAccounts(t *testing.T) {
	f := newFixture(t)
	total, err := transactions.CashOnHand(context.Background(), f.conn, nil)
	if err != nil {
		t.Fatalf("CashOnHand: %v", err)
	}
	if !total.IsZero() {
		t.Errorf("CashOnHand = %s, want 0", total)
	}
}
