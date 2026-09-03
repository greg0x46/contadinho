package transactions_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/categories"
	"contadinho-go/internal/money"
	"contadinho-go/internal/transactions"
)

func TestCreateManualInsertsAnOriginManualRow(t *testing.T) {
	f := newFixture(t)
	accountID := f.addAccount(account{CurrencyCode: strPtr("BRL")})

	id, err := transactions.CreateManual(context.Background(), f.conn, transactions.ManualInput{
		AccountID:   accountID,
		Description: "Almoço",
		Amount:      decimal.RequireFromString("-45.50"),
		OccurredAt:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}

	item, found, err := transactions.GetItem(context.Background(), f.conn, id)
	if err != nil || !found {
		t.Fatalf("GetItem: found=%v err=%v", found, err)
	}
	if item.Origin != "manual" {
		t.Errorf("Origin = %q, want manual", item.Origin)
	}
	if item.Classification != "outflow" {
		t.Errorf("Classification = %q, want outflow", item.Classification)
	}
	if item.ExternalID != "" {
		t.Errorf("ExternalID = %q, want empty for a manual row", item.ExternalID)
	}
	if !item.TotalsEligibility.Included {
		t.Errorf("expected a manual row to be included in totals by default")
	}
}

func TestCreateManualRejectsUnknownAccount(t *testing.T) {
	f := newFixture(t)
	_, err := transactions.CreateManual(context.Background(), f.conn, transactions.ManualInput{
		AccountID:   "does-not-exist",
		Description: "x",
		Amount:      decimal.RequireFromString("10"),
		OccurredAt:  time.Now(),
	})
	if !errors.Is(err, transactions.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestUpdateManualRefusesASyncedRow(t *testing.T) {
	f := newFixture(t)
	accountID := f.addAccount(account{CurrencyCode: strPtr("BRL")})
	syncedID := f.addTransaction(txn{AccountID: accountID})

	err := transactions.UpdateManual(context.Background(), f.conn, syncedID, transactions.ManualInput{
		AccountID:   accountID,
		Description: "tentativa",
		Amount:      decimal.RequireFromString("1"),
		OccurredAt:  time.Now(),
	})
	if !errors.Is(err, transactions.ErrNotManual) {
		t.Fatalf("err = %v, want ErrNotManual", err)
	}
}

func TestUpdateManualChangesTheRow(t *testing.T) {
	f := newFixture(t)
	accountID := f.addAccount(account{CurrencyCode: strPtr("BRL")})
	id, err := transactions.CreateManual(context.Background(), f.conn, transactions.ManualInput{
		AccountID:   accountID,
		Description: "original",
		Amount:      decimal.RequireFromString("-10"),
		OccurredAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}

	if err := transactions.UpdateManual(context.Background(), f.conn, id, transactions.ManualInput{
		AccountID:   accountID,
		Description: "corrigido",
		Amount:      decimal.RequireFromString("20"),
		OccurredAt:  time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("UpdateManual: %v", err)
	}

	item, found, err := transactions.GetItem(context.Background(), f.conn, id)
	if err != nil || !found {
		t.Fatalf("GetItem: found=%v err=%v", found, err)
	}
	if item.Description == nil || *item.Description != "corrigido" {
		t.Errorf("Description = %v, want corrigido", item.Description)
	}
	if item.Classification != "inflow" {
		t.Errorf("Classification = %q, want inflow after flipping the sign", item.Classification)
	}
}

func TestDeleteManualRemovesTheRow(t *testing.T) {
	f := newFixture(t)
	accountID := f.addAccount(account{CurrencyCode: strPtr("BRL")})
	id, err := transactions.CreateManual(context.Background(), f.conn, transactions.ManualInput{
		AccountID:   accountID,
		Description: "apagar",
		Amount:      decimal.RequireFromString("-5"),
		OccurredAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}

	if err := transactions.DeleteManual(context.Background(), f.conn, id, nil); err != nil {
		t.Fatalf("DeleteManual: %v", err)
	}

	if _, found, err := transactions.GetItem(context.Background(), f.conn, id); err != nil || found {
		t.Fatalf("expected the row to be gone: found=%v err=%v", found, err)
	}
}

// TestDeleteManualWorksAfterCategorizationAndIgnoring is the regression test
// for why DeleteManual soft-deletes instead of issuing a hard DELETE:
// transaction_category_events/transaction_inclusion_events are append-only
// and RESTRICT deleting the financial_transactions row they point at, so a
// manual row that was ever categorized or ignored — which is the normal
// case, not an edge one — would be permanently stuck if delete tried to
// remove the base row outright.
func TestDeleteManualWorksAfterCategorizationAndIgnoring(t *testing.T) {
	f := newFixture(t)
	accountID := f.addAccount(account{CurrencyCode: strPtr("BRL")})
	id, err := transactions.CreateManual(context.Background(), f.conn, transactions.ManualInput{
		AccountID:   accountID,
		Description: "categorizado e ignorado",
		Amount:      decimal.RequireFromString("-5"),
		OccurredAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}

	cats, err := categories.List(context.Background(), f.conn)
	if err != nil || len(cats) == 0 {
		t.Fatalf("List categories: err=%v len=%d", err, len(cats))
	}
	if _, err := categories.AssignManual(context.Background(), f.conn, id, cats[0].ID); err != nil {
		t.Fatalf("AssignManual: %v", err)
	}
	if _, err := transactions.SetInclusion(
		context.Background(), f.conn, id, money.Ignored, transactions.InclusionOriginManual, nil, nil, nil,
	); err != nil {
		t.Fatalf("SetInclusion: %v", err)
	}

	if err := transactions.DeleteManual(context.Background(), f.conn, id, nil); err != nil {
		t.Fatalf("DeleteManual: %v", err)
	}
	if _, found, err := transactions.GetItem(context.Background(), f.conn, id); err != nil || found {
		t.Fatalf("expected the row to be gone: found=%v err=%v", found, err)
	}
}

func TestDeleteManualRefusesASyncedRow(t *testing.T) {
	f := newFixture(t)
	accountID := f.addAccount(account{CurrencyCode: strPtr("BRL")})
	syncedID := f.addTransaction(txn{AccountID: accountID})

	if err := transactions.DeleteManual(context.Background(), f.conn, syncedID, nil); !errors.Is(err, transactions.ErrNotManual) {
		t.Fatalf("err = %v, want ErrNotManual", err)
	}
}

func strPtr(s string) *string { return &s }
