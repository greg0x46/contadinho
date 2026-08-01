package syncsvc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"contadinho-go/internal/categories"
	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
	"contadinho-go/internal/pluggy"
)

// safeMessages mirrors SAFE_MESSAGES: user-facing text never leaks raw
// provider errors, only one of these fixed, safe strings.
var safeMessages = map[string]string{
	"invalid_provider_credentials": "Financial provider credentials are invalid",
	"provider_unavailable":         "Financial provider is temporarily unavailable",
	"item_invalid_credentials":     "The financial connection requires updated credentials",
	"item_unavailable":             "The financial connection has no safely updated products",
	"provider_partial_result":      "The provider returned only part of the requested data",
	"provider_rate_limited":        "Financial provider rate limit was exceeded",
	"provider_request_failed":      "Financial provider rejected a read request",
	"invalid_provider_payload":     "The provider returned data that could not be processed safely",
	"unsafe_account_association":   "A transaction could not be linked safely to its account",
	"account_transactions_failed":  "Transactions for this account could not be processed",
	"internal_error":               "Synchronization stopped because of an internal error",
	"interrupted":                  "Synchronization was interrupted before completion",
	"worker_unavailable":           "No worker claimed synchronization before the deadline",
}

// SafeMessage mirrors safe_message.
func SafeMessage(code string) string {
	if msg, ok := safeMessages[code]; ok {
		return msg
	}
	return "Synchronization encountered a safe processing error"
}

// selectFinalStatus mirrors select_final_status.
func selectFinalStatus(hasFailures, safeProgress, generalFailure bool) string {
	if generalFailure || (hasFailures && !safeProgress) {
		return "failed"
	}
	if hasFailures {
		return "completed_with_failures"
	}
	return "completed"
}

// Provider is the subset of *pluggy.Adapter's behavior Service depends on,
// so tests can supply a fake instead of hitting a real Pluggy sandbox.
type Provider interface {
	GetSource(ctx context.Context) (pluggy.SourceSnapshot, string, error)
	GetAccounts(ctx context.Context) (pluggy.AccountsPage, error)
	IterTransactionPages(ctx context.Context, externalAccountID string, handle func(pluggy.TransactionsPage) error) error
}

// TransactionUpsertedHook is called with a transaction's id whenever a sync
// run inserts it for the first time, or updates it because a resync changed
// its normalized hash (see the outcome == "inserted" || outcome == "updated"
// call site in upsertTransaction) — not on "unchanged" outcomes, where
// nothing about the transaction could newly match a rule. It exists so
// package automation (phase 5) can (re-)apply active rules without this
// package importing it — automation's matching logic has no reason to depend
// on sync, so the dependency only goes one way once phase 5 wires a real
// hook in; nil is a valid no-op for now.
type TransactionUpsertedHook func(ctx context.Context, conn *sql.DB, transactionID, accountID string) error

// Service mirrors SyncService.
type Service struct {
	DB                    *sql.DB
	Provider              Provider
	SyncRunID             string
	SourceID              string
	OnTransactionUpserted TransactionUpsertedHook
}

// Execute mirrors SyncService.execute.
func (s *Service) Execute(ctx context.Context) error {
	source, itemRawImportID, err := s.Provider.GetSource(ctx)
	if err != nil {
		var provErr *pluggy.ProviderError
		if errors.As(err, &provErr) {
			return s.failGeneral(ctx, provErr)
		}
		log.Printf("sync_run_internal_error run_id=%s: %v", s.SyncRunID, err)
		return s.failGeneral(ctx, &pluggy.ProviderError{Code: "internal_error", Stage: pluggy.StageAccount, SafeMessage: SafeMessage("internal_error")})
	}
	if err := s.updateSourceName(ctx, source.InstitutionName); err != nil {
		return err
	}
	for _, warning := range source.Warnings {
		if err := s.recordFailure(ctx, pluggy.StageItem, warning, &itemRawImportID, nil, nil); err != nil {
			return err
		}
	}
	if !source.SafeProducts["ACCOUNTS"] {
		return s.failGeneral(ctx, &pluggy.ProviderError{Code: "item_unavailable", Stage: pluggy.StageItem, SafeMessage: SafeMessage("item_unavailable"), RawImportID: &itemRawImportID})
	}
	transactionsSafe := source.SafeProducts["TRANSACTIONS"]

	accountsPage, err := s.Provider.GetAccounts(ctx)
	if err != nil {
		var provErr *pluggy.ProviderError
		if errors.As(err, &provErr) {
			return s.failGeneral(ctx, provErr)
		}
		log.Printf("sync_run_internal_error run_id=%s: %v", s.SyncRunID, err)
		return s.failGeneral(ctx, &pluggy.ProviderError{Code: "internal_error", Stage: pluggy.StageAccount, SafeMessage: SafeMessage("internal_error")})
	}

	for _, rejection := range accountsPage.Rejections {
		if err := s.recordRejection(ctx, rejection, accountsPage.RawImportID, nil, nil); err != nil {
			return err
		}
	}

	for _, record := range accountsPage.Accounts {
		accountID, err := s.upsertAccount(ctx, record, accountsPage.RawImportID)
		if err != nil {
			log.Printf("account_persistence_failed run_id=%s account=%s: %v", s.SyncRunID, record.ExternalID, err)
			extID := record.ExternalID
			if ferr := s.recordFailure(ctx, pluggy.StageAccount, "internal_error", nil, nil, &extID); ferr != nil {
				return ferr
			}
			continue
		}
		if !transactionsSafe {
			continue
		}
		if err := s.incrementAccountsProcessed(ctx); err != nil {
			return err
		}

		extID := record.ExternalID
		err = s.Provider.IterTransactionPages(ctx, record.ExternalID, func(page pluggy.TransactionsPage) error {
			return s.processTransactionPage(ctx, accountID, record.ExternalID, page)
		})
		if err != nil {
			var provErr *pluggy.ProviderError
			if errors.As(err, &provErr) {
				code := "account_transactions_failed"
				if provErr.Code == "provider_rate_limited" || provErr.Code == "invalid_provider_payload" {
					code = provErr.Code
				}
				if ferr := s.recordFailure(ctx, pluggy.StageTransactions, code, provErr.RawImportID, &accountID, &extID); ferr != nil {
					return ferr
				}
			} else {
				log.Printf("account_processing_failed run_id=%s account=%s: %v", s.SyncRunID, record.ExternalID, err)
				if ferr := s.recordFailure(ctx, pluggy.StageTransactions, "account_transactions_failed", nil, &accountID, &extID); ferr != nil {
					return ferr
				}
			}
		}
	}
	return s.finalize(ctx)
}

func (s *Service) updateSourceName(ctx context.Context, displayName *string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE data_sources SET display_name = ?, updated_at = ? WHERE id = ?`,
		displayName, db.FormatTime(time.Now()), s.SourceID)
	return err
}

// decimalToStorage renders an optional decimal for a TEXT column, preserving
// its own scale (see money.CanonicalDecimal) rather than losing trailing
// zeros — nil stays SQL NULL.
func decimalToStorage(value *decimal.Decimal) any {
	if value == nil {
		return nil
	}
	return money.CanonicalDecimal(*value)
}

func (s *Service) upsertAccount(ctx context.Context, snapshot pluggy.AccountSnapshot, rawImportID string) (string, error) {
	digest := pluggy.AccountHash(snapshot)
	now := db.FormatTime(time.Now())

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	newID := uuid.NewString()
	var insertedID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO financial_accounts (
			id, source_id, external_id, institution, name, number, account_type,
			account_subtype, balance, credit_limit, currency_code, provider_updated_at,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (source_id, external_id) DO NOTHING
		RETURNING id`,
		newID, s.SourceID, snapshot.ExternalID, snapshot.Institution, snapshot.Name, snapshot.Number,
		snapshot.AccountType, snapshot.AccountSubtype, decimalToStorage(snapshot.Balance), decimalToStorage(snapshot.CreditLimit),
		snapshot.CurrencyCode, db.FormatTimePtr(snapshot.ProviderUpdatedAt), rawImportID, digest, now, now,
	).Scan(&insertedID)
	inserted := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	var accountID, existingHash string
	if err := tx.QueryRowContext(ctx, `SELECT id, normalized_hash FROM financial_accounts WHERE source_id = ? AND external_id = ?`,
		s.SourceID, snapshot.ExternalID).Scan(&accountID, &existingHash); err != nil {
		return "", err
	}

	outcome := "unchanged"
	switch {
	case inserted:
		outcome = "inserted"
	case existingHash != digest:
		outcome = "updated"
		_, err = tx.ExecContext(ctx, `
			UPDATE financial_accounts SET institution = ?, name = ?, number = ?, account_type = ?,
				account_subtype = ?, balance = ?, credit_limit = ?, currency_code = ?,
				provider_updated_at = ?, current_raw_import_id = ?, normalized_hash = ?, updated_at = ?
			WHERE id = ?`,
			snapshot.Institution, snapshot.Name, snapshot.Number, snapshot.AccountType,
			snapshot.AccountSubtype, decimalToStorage(snapshot.Balance), decimalToStorage(snapshot.CreditLimit),
			snapshot.CurrencyCode, db.FormatTimePtr(snapshot.ProviderUpdatedAt), rawImportID, digest, now, accountID,
		)
		if err != nil {
			return "", err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO normalization_events (id, sync_run_id, raw_import_id, entity_type, account_id, external_id, outcome, normalized_hash, created_at)
		VALUES (?, ?, ?, 'account', ?, ?, ?, ?, ?)`,
		uuid.NewString(), s.SyncRunID, rawImportID, accountID, snapshot.ExternalID, outcome, digest, now,
	); err != nil {
		return "", err
	}
	return accountID, tx.Commit()
}

func (s *Service) incrementAccountsProcessed(ctx context.Context) error {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE sync_runs SET accounts_processed = accounts_processed + 1 WHERE id = ? AND status = 'in_progress'`,
		s.SyncRunID)
	if err != nil {
		return err
	}
	_, err = res.RowsAffected()
	return err
}

func (s *Service) processTransactionPage(ctx context.Context, accountID, externalAccountID string, page pluggy.TransactionsPage) error {
	for _, rejection := range page.Rejections {
		if err := s.recordRejection(ctx, rejection, page.RawImportID, &accountID, &externalAccountID); err != nil {
			return err
		}
	}
	for _, record := range page.Transactions {
		if err := s.upsertTransaction(ctx, accountID, record, page.RawImportID); err != nil {
			extID := record.ExternalID
			if rerr := s.recordRejection(ctx, pluggy.RejectedRecord{
				EntityType: "transaction", ExternalID: &extID, Code: "internal_error", SafeMessage: SafeMessage("internal_error"),
			}, page.RawImportID, &accountID, &externalAccountID); rerr != nil {
				return rerr
			}
		}
	}
	return nil
}

// upsertTransaction runs in its own transaction per record — a deliberate
// simplification of the reference's per-page transaction with a savepoint
// per record: SQLite's single-writer-connection model (see internal/db)
// already serializes every transaction here, so per-record transactions give
// the same "one bad record doesn't corrupt its neighbors" guarantee with far
// simpler code, at the cost of one more commit per record than batching
// would need — negligible at a single user's transaction volume.
func (s *Service) upsertTransaction(ctx context.Context, accountID string, snapshot pluggy.TransactionSnapshot, rawImportID string) error {
	digest := pluggy.TransactionHash(snapshot)
	now := db.FormatTime(time.Now())

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	newID := uuid.NewString()
	var insertedID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description, description_raw, amount,
			amount_in_account_currency, balance_after, currency_code, occurred_at, provider_status,
			movement_type, source_category, source_category_id, provider_code, payment_data,
			credit_card_metadata, merchant, operation_type, operation_type_additional_info,
			operation_category, provider_id, provider_order, provider_created_at, provider_updated_at,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (source_id, external_id) DO NOTHING
		RETURNING id`,
		newID, s.SourceID, accountID, snapshot.ExternalID, snapshot.Description, snapshot.DescriptionRaw,
		decimalToStorage(snapshot.Amount), decimalToStorage(snapshot.AmountInAccountCurrency), decimalToStorage(snapshot.BalanceAfter),
		snapshot.CurrencyCode, db.FormatTimePtr(snapshot.OccurredAt), snapshot.ProviderStatus, snapshot.MovementType,
		snapshot.SourceCategory, snapshot.SourceCategoryID, snapshot.ProviderCode, nullBytes(snapshot.PaymentData),
		nullBytes(snapshot.CreditCardMetadata), nullBytes(snapshot.Merchant), snapshot.OperationType,
		snapshot.OperationTypeAdditionalInfo, snapshot.OperationCategory, snapshot.ProviderID, snapshot.ProviderOrder,
		db.FormatTimePtr(snapshot.ProviderCreatedAt), db.FormatTimePtr(snapshot.ProviderUpdatedAt),
		rawImportID, digest, now, now,
	).Scan(&insertedID)
	inserted := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var transactionID, existingHash string
	if err := tx.QueryRowContext(ctx, `SELECT id, normalized_hash FROM financial_transactions WHERE source_id = ? AND external_id = ?`,
		s.SourceID, snapshot.ExternalID).Scan(&transactionID, &existingHash); err != nil {
		return err
	}

	outcome := "unchanged"
	switch {
	case inserted:
		outcome = "inserted"
	case existingHash != digest:
		outcome = "updated"
		_, err = tx.ExecContext(ctx, `
			UPDATE financial_transactions SET description = ?, description_raw = ?, amount = ?,
				amount_in_account_currency = ?, balance_after = ?, currency_code = ?, occurred_at = ?,
				provider_status = ?, movement_type = ?, source_category = ?, source_category_id = ?,
				provider_code = ?, payment_data = ?, credit_card_metadata = ?, merchant = ?,
				operation_type = ?, operation_type_additional_info = ?, operation_category = ?,
				provider_id = ?, provider_order = ?, provider_created_at = ?, provider_updated_at = ?,
				current_raw_import_id = ?, normalized_hash = ?, updated_at = ?
			WHERE id = ?`,
			snapshot.Description, snapshot.DescriptionRaw, decimalToStorage(snapshot.Amount),
			decimalToStorage(snapshot.AmountInAccountCurrency), decimalToStorage(snapshot.BalanceAfter), snapshot.CurrencyCode,
			db.FormatTimePtr(snapshot.OccurredAt), snapshot.ProviderStatus, snapshot.MovementType, snapshot.SourceCategory,
			snapshot.SourceCategoryID, snapshot.ProviderCode, nullBytes(snapshot.PaymentData), nullBytes(snapshot.CreditCardMetadata),
			nullBytes(snapshot.Merchant), snapshot.OperationType, snapshot.OperationTypeAdditionalInfo, snapshot.OperationCategory,
			snapshot.ProviderID, snapshot.ProviderOrder, db.FormatTimePtr(snapshot.ProviderCreatedAt), db.FormatTimePtr(snapshot.ProviderUpdatedAt),
			rawImportID, digest, now, transactionID,
		)
		if err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO normalization_events (id, sync_run_id, raw_import_id, entity_type, transaction_id, external_id, outcome, normalized_hash, created_at)
		VALUES (?, ?, ?, 'transaction', ?, ?, ?, ?, ?)`,
		uuid.NewString(), s.SyncRunID, rawImportID, transactionID, snapshot.ExternalID, outcome, digest, now,
	); err != nil {
		return err
	}

	if outcome == "inserted" || outcome == "updated" {
		column := "transactions_updated"
		if outcome == "inserted" {
			column = "transactions_inserted"
		}
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`UPDATE sync_runs SET %s = %s + 1 WHERE id = ? AND status = 'in_progress'`, column, column),
			s.SyncRunID,
		); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// categories.ApplyAutomatic stays insert-only: it already no-ops whenever
	// a category decision exists for the transaction (see
	// categories.ApplyAutomatic), so re-running it on every update would be
	// safe, but nothing reported forces that scope here — only the
	// automation-rule staleness case below has an observed bug, so widening
	// categorization's trigger is left for whoever actually needs it.
	if outcome == "inserted" {
		if err := categories.ApplyAutomatic(ctx, s.DB, transactionID, snapshot.SourceCategory); err != nil {
			return err
		}
	}
	if outcome == "inserted" || outcome == "updated" {
		if s.OnTransactionUpserted != nil {
			if err := s.OnTransactionUpserted(ctx, s.DB, transactionID, accountID); err != nil {
				return err
			}
		}
	}
	return nil
}

func nullBytes(b []byte) any {
	if b == nil {
		return nil
	}
	return string(b)
}

func (s *Service) recordRejection(ctx context.Context, rejection pluggy.RejectedRecord, rawImportID string, accountID, externalAccountID *string) error {
	now := db.FormatTime(time.Now())
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO normalization_events (id, sync_run_id, raw_import_id, entity_type, external_id, outcome, created_at)
		VALUES (?, ?, ?, ?, ?, 'rejected', ?)`,
		uuid.NewString(), s.SyncRunID, rawImportID, rejection.EntityType, rejection.ExternalID, now,
	); err != nil {
		return err
	}
	var externalTransactionID *string
	if rejection.EntityType == "transaction" {
		externalTransactionID = rejection.ExternalID
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sync_failures (
			id, sync_run_id, raw_import_id, account_id, external_account_id,
			external_transaction_id, stage, error_code, safe_message, created_at
		) VALUES (?, ?, ?, ?, ?, ?, 'normalize', ?, ?, ?)`,
		uuid.NewString(), s.SyncRunID, rawImportID, accountID, externalAccountID,
		externalTransactionID, rejection.Code, SafeMessage(rejection.Code), now,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) recordFailure(ctx context.Context, stage pluggy.FailureStage, code string, rawImportID *string, accountID, externalAccountID *string) error {
	now := db.FormatTime(time.Now())
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO sync_failures (id, sync_run_id, raw_import_id, account_id, external_account_id, stage, error_code, safe_message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), s.SyncRunID, rawImportID, accountID, externalAccountID, string(stage), code, SafeMessage(code), now,
	)
	return err
}

func (s *Service) failGeneral(ctx context.Context, provErr *pluggy.ProviderError) error {
	log.Printf("sync_run_general_failure run_id=%s stage=%s code=%s", s.SyncRunID, provErr.Stage, provErr.Code)
	now := db.FormatTime(time.Now())
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM sync_runs WHERE id = ?`, s.SyncRunID).Scan(&status); err != nil {
		return err
	}
	if status != "in_progress" {
		return tx.Commit()
	}
	message := SafeMessage(provErr.Code)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sync_failures (id, sync_run_id, raw_import_id, external_account_id, stage, error_code, safe_message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), s.SyncRunID, provErr.RawImportID, provErr.ExternalAccountID, string(provErr.Stage), provErr.Code, message, now,
	); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE sync_runs SET status = 'failed', finished_at = ?, general_error_code = ?, general_error_message = ?
		WHERE id = ?`,
		now, provErr.Code, message, s.SyncRunID,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) finalize(ctx context.Context) error {
	now := db.FormatTime(time.Now())
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM sync_runs WHERE id = ?`, s.SyncRunID).Scan(&status); err != nil {
		return err
	}
	if status != "in_progress" {
		return tx.Commit()
	}

	var failureCount, safeCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_failures WHERE sync_run_id = ?`, s.SyncRunID).Scan(&failureCount); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM normalization_events WHERE sync_run_id = ? AND outcome <> 'rejected'`, s.SyncRunID).Scan(&safeCount); err != nil {
		return err
	}
	finalStatus := selectFinalStatus(failureCount > 0, safeCount > 0, false)
	if _, err := tx.ExecContext(ctx, `UPDATE sync_runs SET status = ?, finished_at = ? WHERE id = ?`, finalStatus, now, s.SyncRunID); err != nil {
		return err
	}
	log.Printf("sync_run_completed run_id=%s status=%s", s.SyncRunID, finalStatus)
	return tx.Commit()
}
