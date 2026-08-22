package scenarios

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
)

// ErrRealizationNotFound is returned by DeleteRealization when a
// realization id has no matching row.
var ErrRealizationNotFound = errors.New("scenario transaction realization not found")

// CreateRealization mirrors allocating (part of) a
// payable_transaction_links row's amount to scenarioTransactionID.
// Callers are responsible for validating that both ids exist and belong to
// the same payable before calling this — the schema's FKs (ON DELETE
// CASCADE both ways) are the only enforcement at this layer.
func CreateRealization(ctx context.Context, q Querier, scenarioTransactionID string, payableLinkID *string, allocatedAmount decimal.Decimal) (ScenarioTransactionRealization, error) {
	now := time.Now().UTC()
	r := ScenarioTransactionRealization{
		ID: uuid.NewString(), ScenarioTransactionID: scenarioTransactionID, PayableLinkID: payableLinkID,
		AllocatedAmount: allocatedAmount, CreatedAt: now,
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO scenario_transaction_realizations (id, scenario_transaction_id, payable_link_id, allocated_amount, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		r.ID, r.ScenarioTransactionID, r.PayableLinkID, money.CanonicalDecimal(r.AllocatedAmount), db.FormatTime(now),
	)
	if err != nil {
		return ScenarioTransactionRealization{}, err
	}
	return r, nil
}

// GetRealization mirrors reading a single allocation by id — callers that
// need to scope it to a specific scenario_transaction (the HTTP layer's
// delete endpoint) check ScenarioTransactionID themselves, the same
// pattern payables.GetLink/DeleteLink uses.
func GetRealization(ctx context.Context, q Querier, id string) (ScenarioTransactionRealization, error) {
	var r ScenarioTransactionRealization
	var payableLinkID sql.NullString
	var allocatedAmountRaw, createdAtRaw string
	err := q.QueryRowContext(ctx,
		`SELECT id, scenario_transaction_id, payable_link_id, allocated_amount, created_at FROM scenario_transaction_realizations WHERE id = ?`, id,
	).Scan(&r.ID, &r.ScenarioTransactionID, &payableLinkID, &allocatedAmountRaw, &createdAtRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return ScenarioTransactionRealization{}, ErrRealizationNotFound
	}
	if err != nil {
		return ScenarioTransactionRealization{}, err
	}
	if payableLinkID.Valid {
		r.PayableLinkID = &payableLinkID.String
	}
	if r.AllocatedAmount, err = decimal.NewFromString(allocatedAmountRaw); err != nil {
		return ScenarioTransactionRealization{}, err
	}
	if r.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
		return ScenarioTransactionRealization{}, err
	}
	return r, nil
}

// DeleteRealization mirrors removing a single allocation, undoing it
// without touching the scenario_transaction or the debt_transaction_link
// it referenced.
func DeleteRealization(ctx context.Context, q Querier, id string) error {
	res, err := q.ExecContext(ctx, `DELETE FROM scenario_transaction_realizations WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRealizationNotFound
	}
	return nil
}

// sumAllocated mirrors the spec's realizedTotal: the sum of
// allocated_amount across every realization allocated to one installment,
// computed on read — never stored on the scenario_transaction row itself.
// Summed in Go with decimal.Decimal (matching debts.PaidAmount's approach)
// rather than SQL SUM, which would have to go through a float-lossy
// CAST(... AS REAL) on this driver.
//
// It takes the rows rather than an id because every caller has already
// loaded them in bulk via realizationsFor — a per-installment query for
// this number is exactly the round trip that batching removed.
func sumAllocated(list []ScenarioTransactionRealization) decimal.Decimal {
	total := decimal.Zero
	for _, r := range list {
		total = total.Add(r.AllocatedAmount)
	}
	return total
}

// listRealizationsForTransaction mirrors listing every allocation a planned
// installment has received, newest first.
func listRealizationsForTransaction(ctx context.Context, q Querier, scenarioTransactionID string) ([]ScenarioTransactionRealization, error) {
	byTransaction, err := realizationsFor(ctx, q, []string{scenarioTransactionID})
	if err != nil {
		return nil, err
	}
	return byTransaction[scenarioTransactionID], nil
}

// realizationsFor loads every allocation belonging to any of
// scenarioTransactionIDs in one query, keyed by scenario_transaction_id and
// newest-first within each key.
//
// It exists because every interesting read here is per-scenario, not
// per-installment: summarizing a scenario, listing a plan's unrealized
// installments, deciding which installments a readjustment may replace. One
// query for the whole set keeps those O(1) in round trips instead of O(number
// of installments). An installment with no allocations is simply absent from
// the map.
func realizationsFor(ctx context.Context, q Querier, scenarioTransactionIDs []string) (map[string][]ScenarioTransactionRealization, error) {
	result := make(map[string][]ScenarioTransactionRealization, len(scenarioTransactionIDs))
	if len(scenarioTransactionIDs) == 0 {
		return result, nil
	}
	in, args := db.InClause(scenarioTransactionIDs)
	rows, err := q.QueryContext(ctx, `
		SELECT id, scenario_transaction_id, payable_link_id, allocated_amount, created_at
		FROM scenario_transaction_realizations
		WHERE scenario_transaction_id IN (`+in+`)
		ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			r                  ScenarioTransactionRealization
			payableLinkID      sql.NullString
			allocatedAmountRaw string
			createdAtRaw       string
		)
		if err := rows.Scan(&r.ID, &r.ScenarioTransactionID, &payableLinkID, &allocatedAmountRaw, &createdAtRaw); err != nil {
			return nil, err
		}
		if payableLinkID.Valid {
			r.PayableLinkID = &payableLinkID.String
		}
		if r.AllocatedAmount, err = decimal.NewFromString(allocatedAmountRaw); err != nil {
			return nil, err
		}
		if r.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
			return nil, err
		}
		result[r.ScenarioTransactionID] = append(result[r.ScenarioTransactionID], r)
	}
	return result, rows.Err()
}
