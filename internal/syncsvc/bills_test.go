package syncsvc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"contadinho-go/internal/pluggy"
	"contadinho-go/internal/syncsvc"
)

// TestExecuteUpsertsBillsForCreditAccount confirms a credit card account's
// closed bills are synced into financial_bills and counted, alongside its
// accounts/transactions.
func TestExecuteUpsertsBillsForCreditAccount(t *testing.T) {
	conn := newTestConn(t)
	sourceID, syncRunID := newSyncRun(t, conn)
	insertRawImport(t, conn, "raw-item", syncRunID, sourceID)
	insertRawImport(t, conn, "raw-accounts", syncRunID, sourceID)
	insertRawImport(t, conn, "raw-bills-1", syncRunID, sourceID)

	dueDate := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	provider := &fakeProvider{
		source:            defaultSource(),
		sourceRawImportID: "raw-item",
		accountsPage: pluggy.AccountsPage{
			RawImportID: "raw-accounts",
			Accounts:    []pluggy.AccountSnapshot{{ExternalID: "acc-credit", CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")}},
		},
		bills: map[string]pluggy.BillsPage{
			"acc-credit": {
				RawImportID: "raw-bills-1",
				Bills:       []pluggy.BillSnapshot{{ExternalID: "bill-1", DueDate: &dueDate}},
			},
		},
	}

	service := &syncsvc.Service{DB: conn, Provider: provider, SyncRunID: syncRunID, SourceID: sourceID}
	if err := service.Execute(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var billsProcessed int
	if err := conn.QueryRow(`SELECT bills_processed FROM sync_runs WHERE id = ?`, syncRunID).Scan(&billsProcessed); err != nil {
		t.Fatalf("query bills_processed: %v", err)
	}
	if billsProcessed != 1 {
		t.Errorf("bills_processed = %d, want 1", billsProcessed)
	}

	var externalID, storedDueDate string
	err := conn.QueryRow(`
		SELECT fb.external_id, fb.due_date FROM financial_bills fb
		JOIN financial_accounts fa ON fa.id = fb.account_id
		WHERE fa.external_id = 'acc-credit'`).Scan(&externalID, &storedDueDate)
	if err != nil {
		t.Fatalf("query financial_bills: %v", err)
	}
	if externalID != "bill-1" || storedDueDate[:10] != "2026-04-10" {
		t.Errorf("externalID=%s dueDate=%s", externalID, storedDueDate)
	}
}

// TestExecuteOnlySyncsBillsForCreditAccounts confirms processBills is gated
// on account_type == CREDIT: a non-credit account never calls GetBills at
// all (not just "calls it and gets nothing back"), verified here by forcing
// every GetBills call to fail and checking only the credit account's
// external_account_id shows up in sync_failures.
func TestExecuteOnlySyncsBillsForCreditAccounts(t *testing.T) {
	conn := newTestConn(t)
	sourceID, syncRunID := newSyncRun(t, conn)
	insertRawImport(t, conn, "raw-item", syncRunID, sourceID)
	insertRawImport(t, conn, "raw-accounts", syncRunID, sourceID)

	provider := &fakeProvider{
		source:            defaultSource(),
		sourceRawImportID: "raw-item",
		accountsPage: pluggy.AccountsPage{
			RawImportID: "raw-accounts",
			Accounts: []pluggy.AccountSnapshot{
				{ExternalID: "acc-credit", CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")},
				{ExternalID: "acc-checking", CurrencyCode: strp("BRL"), AccountType: strp("BANK")},
			},
		},
		billsErr: errors.New("bills unavailable"),
	}

	service := &syncsvc.Service{DB: conn, Provider: provider, SyncRunID: syncRunID, SourceID: sourceID}
	if err := service.Execute(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	rows, err := conn.Query(`SELECT external_account_id FROM sync_failures WHERE stage = 'bills'`)
	if err != nil {
		t.Fatalf("query sync_failures: %v", err)
	}
	defer rows.Close()
	var accounts []string
	for rows.Next() {
		var accountID string
		if err := rows.Scan(&accountID); err != nil {
			t.Fatalf("scan: %v", err)
		}
		accounts = append(accounts, accountID)
	}
	if len(accounts) != 1 || accounts[0] != "acc-credit" {
		t.Errorf("bills failures = %+v, want exactly one for acc-credit", accounts)
	}
}
