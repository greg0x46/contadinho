package transactions_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"contadinho-go/internal/money"
	"contadinho-go/internal/transactions"
)

func TestSetInclusionTogglesAndRecordsRevisions(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{CurrencyCode: strp("BRL")})
	occurred := time.Now().UTC()
	txID := f.addTransaction(txn{AccountID: acc, Amount: strp("-1.00"), AmountInAccountCurrency: strp("-1.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})
	ctx := context.Background()

	c1, err := transactions.SetInclusion(ctx, f.conn, txID, money.Ignored, transactions.InclusionOriginManual, nil, nil, nil)
	if err != nil {
		t.Fatalf("SetInclusion: %v", err)
	}
	if !c1.Applied || !c1.Changed || c1.State != money.Ignored {
		t.Errorf("first toggle = %+v", c1)
	}

	c2, err := transactions.SetInclusion(ctx, f.conn, txID, money.Considered, transactions.InclusionOriginManual, nil, nil, nil)
	if err != nil {
		t.Fatalf("SetInclusion: %v", err)
	}
	if !c2.Changed || c2.State != money.Considered {
		t.Errorf("second toggle = %+v", c2)
	}

	var revision int
	if err := f.conn.QueryRow(`SELECT revision FROM transaction_inclusion_decisions WHERE transaction_id = ?`, txID).Scan(&revision); err != nil {
		t.Fatalf("query revision: %v", err)
	}
	if revision != 2 {
		t.Errorf("revision = %d, want 2", revision)
	}

	var eventCount int
	if err := f.conn.QueryRow(`SELECT COUNT(*) FROM transaction_inclusion_events WHERE transaction_id = ?`, txID).Scan(&eventCount); err != nil {
		t.Fatalf("query events: %v", err)
	}
	if eventCount != 2 {
		t.Errorf("event count = %d, want 2", eventCount)
	}
}

func TestSetInclusionIsIdempotent(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{CurrencyCode: strp("BRL")})
	txID := f.addTransaction(txn{AccountID: acc, MovementType: strp("DEBIT")})
	ctx := context.Background()

	c1, err := transactions.SetInclusion(ctx, f.conn, txID, money.Ignored, transactions.InclusionOriginManual, nil, nil, nil)
	if err != nil {
		t.Fatalf("SetInclusion: %v", err)
	}
	c2, err := transactions.SetInclusion(ctx, f.conn, txID, money.Ignored, transactions.InclusionOriginManual, nil, nil, nil)
	if err != nil {
		t.Fatalf("SetInclusion (repeat): %v", err)
	}
	if c2.Changed {
		t.Errorf("repeating the same state should not report Changed=true")
	}
	if c2.ChangedAt == nil || !c2.ChangedAt.Equal(*c1.ChangedAt) {
		t.Errorf("ChangedAt should be preserved across a no-op, got %v want %v", c2.ChangedAt, c1.ChangedAt)
	}
}

func TestSetInclusionManualTakesPrecedenceOverRule(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{CurrencyCode: strp("BRL")})
	txID := f.addTransaction(txn{AccountID: acc, MovementType: strp("DEBIT")})
	ctx := context.Background()

	ruleID := f.addAutomationRule("Ignorar transferências")
	ruleName := "Ignorar transferências"
	if _, err := transactions.SetInclusion(ctx, f.conn, txID, money.Ignored, transactions.InclusionOriginManual, nil, nil, nil); err != nil {
		t.Fatalf("manual SetInclusion: %v", err)
	}

	// A rule trying to flip a manually-decided transaction back must be
	// rejected outright (Applied=false), never silently overridden.
	ruleResult, err := transactions.SetInclusion(ctx, f.conn, txID, money.Considered, transactions.InclusionOriginRule, &ruleID, &ruleName, nil)
	if err != nil {
		t.Fatalf("rule SetInclusion: %v", err)
	}
	if ruleResult.Applied {
		t.Errorf("rule attempt over a manual decision should not be applied: %+v", ruleResult)
	}
	if ruleResult.State != money.Ignored {
		t.Errorf("state should remain unchanged at ignored, got %v", ruleResult.State)
	}
}

func TestSetInclusionRuleClaimedAsManualWhenStateAlreadyMatches(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{CurrencyCode: strp("BRL")})
	txID := f.addTransaction(txn{AccountID: acc, MovementType: strp("DEBIT")})
	ctx := context.Background()

	ruleID := f.addAutomationRule("Ignorar transferências")
	ruleName := "Ignorar transferências"
	if _, err := transactions.SetInclusion(ctx, f.conn, txID, money.Ignored, transactions.InclusionOriginRule, &ruleID, &ruleName, nil); err != nil {
		t.Fatalf("rule SetInclusion: %v", err)
	}

	if _, err := transactions.SetInclusion(ctx, f.conn, txID, money.Ignored, transactions.InclusionOriginManual, nil, nil, nil); err != nil {
		t.Fatalf("manual SetInclusion: %v", err)
	}

	var origin string
	var ruleIDCol, ruleNameCol *string
	err := f.conn.QueryRow(
		`SELECT origin, rule_id, rule_name FROM transaction_inclusion_decisions WHERE transaction_id = ?`, txID,
	).Scan(&origin, &ruleIDCol, &ruleNameCol)
	if err != nil {
		t.Fatalf("query decision: %v", err)
	}
	if origin != "manual" || ruleIDCol != nil || ruleNameCol != nil {
		t.Errorf("decision should be claimed as manual with rule fields cleared: origin=%s rule_id=%v rule_name=%v", origin, ruleIDCol, ruleNameCol)
	}
}

func TestSetInclusionUnknownTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := transactions.SetInclusion(ctx, f.conn, "does-not-exist", money.Ignored, transactions.InclusionOriginManual, nil, nil, nil)
	if !errors.Is(err, transactions.ErrTransactionNotFound) {
		t.Errorf("err = %v, want ErrTransactionNotFound", err)
	}
}

func TestSetInclusionInvokesOnIgnoredHook(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{CurrencyCode: strp("BRL")})
	txID := f.addTransaction(txn{AccountID: acc, MovementType: strp("DEBIT")})
	ctx := context.Background()

	var hookCalledWith string
	hook := func(ctx context.Context, q transactions.Querier, transactionID string) error {
		hookCalledWith = transactionID
		return nil
	}
	if _, err := transactions.SetInclusion(ctx, f.conn, txID, money.Ignored, transactions.InclusionOriginManual, nil, nil, hook); err != nil {
		t.Fatalf("SetInclusion: %v", err)
	}
	if hookCalledWith != txID {
		t.Errorf("onIgnored hook called with %q, want %q", hookCalledWith, txID)
	}
}
