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

// CreateRealization mirrors allocating (part of) a debt_transaction_links or
// receivable_transaction_links row's amount to scenarioTransactionID —
// exactly one of debtLinkID/receivableLinkID must be set, matching the
// parent scenario's Kind. Callers are responsible for validating that both
// ids exist and belong to the same debt/receivable before calling this —
// the schema's FKs (ON DELETE CASCADE both ways) are the only enforcement
// at this layer.
func CreateRealization(ctx context.Context, q Querier, scenarioTransactionID string, debtLinkID, receivableLinkID *string, allocatedAmount decimal.Decimal) (ScenarioTransactionRealization, error) {
	now := time.Now().UTC()
	r := ScenarioTransactionRealization{
		ID: uuid.NewString(), ScenarioTransactionID: scenarioTransactionID, DebtLinkID: debtLinkID, ReceivableLinkID: receivableLinkID,
		AllocatedAmount: allocatedAmount, CreatedAt: now,
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO scenario_transaction_realizations (id, scenario_transaction_id, debt_link_id, receivable_link_id, allocated_amount, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.ScenarioTransactionID, r.DebtLinkID, r.ReceivableLinkID, money.CanonicalDecimal(r.AllocatedAmount), db.FormatTime(now),
	)
	if err != nil {
		return ScenarioTransactionRealization{}, err
	}
	return r, nil
}

// GetRealization mirrors reading a single allocation by id — callers that
// need to scope it to a specific scenario_transaction (the HTTP layer's
// delete endpoint) check ScenarioTransactionID themselves, the same
// pattern debts.GetLink/DeleteLink uses.
func GetRealization(ctx context.Context, q Querier, id string) (ScenarioTransactionRealization, error) {
	var r ScenarioTransactionRealization
	var debtLinkID, receivableLinkID sql.NullString
	var allocatedAmountRaw, createdAtRaw string
	err := q.QueryRowContext(ctx,
		`SELECT id, scenario_transaction_id, debt_link_id, receivable_link_id, allocated_amount, created_at FROM scenario_transaction_realizations WHERE id = ?`, id,
	).Scan(&r.ID, &r.ScenarioTransactionID, &debtLinkID, &receivableLinkID, &allocatedAmountRaw, &createdAtRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return ScenarioTransactionRealization{}, ErrRealizationNotFound
	}
	if err != nil {
		return ScenarioTransactionRealization{}, err
	}
	if debtLinkID.Valid {
		r.DebtLinkID = &debtLinkID.String
	}
	if receivableLinkID.Valid {
		r.ReceivableLinkID = &receivableLinkID.String
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

// RealizedTotal mirrors the spec's realizedTotal: the sum of
// allocated_amount across every realization allocated to
// scenarioTransactionID, computed on read — never stored on the
// scenario_transaction row itself. Summed in Go with decimal.Decimal
// (matching debts.PaidAmount's approach) rather than SQL SUM, which would
// have to go through a float-lossy CAST(... AS REAL) on this driver.
func RealizedTotal(ctx context.Context, q Querier, scenarioTransactionID string) (decimal.Decimal, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT allocated_amount FROM scenario_transaction_realizations WHERE scenario_transaction_id = ?`,
		scenarioTransactionID)
	if err != nil {
		return decimal.Zero, err
	}
	defer rows.Close()
	total := decimal.Zero
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return decimal.Zero, err
		}
		amount, err := decimal.NewFromString(raw)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(amount)
	}
	return total, rows.Err()
}

// ListRealizationsForTransaction mirrors listing every allocation a planned
// installment has received, newest first.
func ListRealizationsForTransaction(ctx context.Context, q Querier, scenarioTransactionID string) ([]ScenarioTransactionRealization, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, scenario_transaction_id, debt_link_id, receivable_link_id, allocated_amount, created_at
		FROM scenario_transaction_realizations WHERE scenario_transaction_id = ? ORDER BY created_at DESC`, scenarioTransactionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []ScenarioTransactionRealization
	for rows.Next() {
		var (
			r                            ScenarioTransactionRealization
			debtLinkID, receivableLinkID sql.NullString
			allocatedAmountRaw           string
			createdAtRaw                 string
		)
		if err := rows.Scan(&r.ID, &r.ScenarioTransactionID, &debtLinkID, &receivableLinkID, &allocatedAmountRaw, &createdAtRaw); err != nil {
			return nil, err
		}
		if debtLinkID.Valid {
			r.DebtLinkID = &debtLinkID.String
		}
		if receivableLinkID.Valid {
			r.ReceivableLinkID = &receivableLinkID.String
		}
		if r.AllocatedAmount, err = decimal.NewFromString(allocatedAmountRaw); err != nil {
			return nil, err
		}
		if r.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}
