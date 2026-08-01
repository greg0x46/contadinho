package scenarios

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ErrInvalidInstallmentPlan is returned by GenerateInstallments when
// totalAmount or months can't produce a sensible monthly plan.
var ErrInvalidInstallmentPlan = errors.New("invalid installment plan")

// GeneratedInstallment is one row GenerateInstallments proposes — not yet
// persisted.
type GeneratedInstallment struct {
	Description string
	Amount      decimal.Decimal
	ProjectedAt time.Time
}

// splitEvenly divides totalAmount into n shares at 2-decimal (BRL cents)
// precision: every share but the last is the floor of the even split, and
// the last absorbs whatever floor-division left over — so the shares always
// sum to exactly totalAmount. Shared by GenerateInstallments and the
// readjustment strategies (Readjust), which both need this "last absorbs
// the remainder" rule.
func splitEvenly(totalAmount decimal.Decimal, n int) []decimal.Decimal {
	share := totalAmount.Div(decimal.NewFromInt(int64(n))).Truncate(2)
	shares := make([]decimal.Decimal, n)
	runningTotal := decimal.Zero
	for i := 0; i < n; i++ {
		amount := share
		if i == n-1 {
			amount = totalAmount.Sub(runningTotal)
		}
		shares[i] = amount
		runningTotal = runningTotal.Add(amount)
	}
	return shares
}

// GenerateInstallments splits totalAmount into months equal monthly
// installments, one per calendar month starting at startDate.
func GenerateInstallments(totalAmount decimal.Decimal, months int, startDate time.Time) ([]GeneratedInstallment, error) {
	if !totalAmount.IsPositive() {
		return nil, ErrInvalidInstallmentPlan
	}
	if months < 1 {
		return nil, ErrInvalidInstallmentPlan
	}

	shares := splitEvenly(totalAmount, months)
	installments := make([]GeneratedInstallment, months)
	for i, amount := range shares {
		installments[i] = GeneratedInstallment{
			Description: fmt.Sprintf("Parcela %d/%d", i+1, months),
			Amount:      amount,
			ProjectedAt: startDate.AddDate(0, i, 0),
		}
	}
	return installments, nil
}

// CreateGeneratedInstallments persists every installment GenerateInstallments
// proposed for scenarioID as scenario_transactions, all inside one
// transaction so a mid-batch failure leaves no partial plan behind.
func CreateGeneratedInstallments(ctx context.Context, conn *sql.DB, scenarioID string, installments []GeneratedInstallment) ([]ScenarioTransaction, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	created := make([]ScenarioTransaction, len(installments))
	for i, ins := range installments {
		st, err := CreateScenarioTransaction(ctx, tx, scenarioID, ins.Description, ins.Amount, ins.ProjectedAt, nil)
		if err != nil {
			return nil, err
		}
		created[i] = st
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return created, nil
}
