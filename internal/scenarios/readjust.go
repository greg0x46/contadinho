package scenarios

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/shopspring/decimal"
)

// ReadjustStrategy selects how Readjust regenerates unallocated installments.
type ReadjustStrategy string

const (
	// StrategyReduceTerm ("Abater do final") keeps the installment value
	// (the last affected installment's amount) and lets the number of
	// installments — and so the payoff date — move: fewer if the plan is
	// ahead, more if it's behind.
	StrategyReduceTerm ReadjustStrategy = "abater_do_final"
	// StrategyRedistribute ("Redistribuir entre os meses") keeps the same
	// number of installments (and so the same payoff date) and lets the
	// per-installment value move up or down to absorb the deviation.
	StrategyRedistribute ReadjustStrategy = "redistribuir"
)

// ErrNoAffectedInstallments is returned by Readjust when every installment
// already has some allocation (realizedTotal > 0) — there is nothing left
// for a readjustment to replace.
var ErrNoAffectedInstallments = errors.New("no unallocated installments to readjust")

// ErrNothingToRedistribute is returned by Readjust when the balance left to
// redistribute (remainingAmount minus what's already reserved by partially
// realized installments) isn't positive — regenerating installments for a
// zero or negative balance would only produce zero or negative amounts, so
// Readjust refuses rather than doing that.
var ErrNothingToRedistribute = errors.New("nothing left to redistribute")

// ErrInvalidStrategy is returned by Readjust for any strategy value other
// than the two defined constants.
var ErrInvalidStrategy = errors.New("invalid readjustment strategy")

// Readjust mirrors the spec's "Reajustar parcelas restantes": it replaces
// every installment with realizedTotal = 0 (no allocation at all, whether
// "projetada" or "atrasada") with a fresh set that sums to
// remainingAmount − Σ(amount − realizedTotal) of the installments left
// untouched — every installment with *any* allocation, even partial, is
// preserved exactly as-is and never touched here. remainingAmount is the
// caller-supplied debts.RemainingAmount for the scenario's debt; this
// package has no dependency on internal/debts, so it's passed in rather
// than recomputed here.
func Readjust(ctx context.Context, conn *sql.DB, scenarioID string, remainingAmount decimal.Decimal, strategy ReadjustStrategy) ([]ScenarioTransaction, error) {
	if strategy != StrategyReduceTerm && strategy != StrategyRedistribute {
		return nil, ErrInvalidStrategy
	}

	list, err := ListScenarioTransactions(ctx, conn, scenarioID)
	if err != nil {
		return nil, err
	}

	var affected []ScenarioTransaction
	reserved := decimal.Zero
	for _, st := range list {
		realizedTotal, err := RealizedTotal(ctx, conn, st.ID)
		if err != nil {
			return nil, err
		}
		if realizedTotal.IsZero() {
			affected = append(affected, st)
		} else {
			reserved = reserved.Add(st.Amount.Sub(realizedTotal))
		}
	}
	if len(affected) == 0 {
		return nil, ErrNoAffectedInstallments
	}

	balance := remainingAmount.Sub(reserved)
	if !balance.IsPositive() {
		return nil, ErrNothingToRedistribute
	}

	sort.Slice(affected, func(i, j int) bool { return affected[i].ProjectedAt.Before(affected[j].ProjectedAt) })

	var drafts []GeneratedInstallment
	switch strategy {
	case StrategyReduceTerm:
		drafts = reduceTermInstallments(balance, affected)
	case StrategyRedistribute:
		drafts = redistributeInstallments(balance, affected)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, st := range affected {
		if err := DeleteScenarioTransaction(ctx, tx, st.ID); err != nil {
			return nil, err
		}
	}
	created := make([]ScenarioTransaction, len(drafts))
	for i, d := range drafts {
		st, err := CreateScenarioTransaction(ctx, tx, scenarioID, d.Description, d.Amount, d.ProjectedAt, nil)
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

// reduceTermInstallments implements "abater do final": every new
// installment is worth referenceAmount (the last affected installment's
// amount before the readjustment) except the last, which absorbs whatever
// floor-division left over; the count is however many of those it takes to
// cover balance, starting from the earliest affected installment's date.
func reduceTermInstallments(balance decimal.Decimal, affected []ScenarioTransaction) []GeneratedInstallment {
	referenceAmount := affected[len(affected)-1].Amount
	startDate := affected[0].ProjectedAt

	amounts := splitByAmount(balance, referenceAmount)
	n := len(amounts)
	installments := make([]GeneratedInstallment, n)
	for i, amount := range amounts {
		installments[i] = GeneratedInstallment{
			Description: fmt.Sprintf("Parcela %d/%d (reajustada)", i+1, n),
			Amount:      amount,
			ProjectedAt: startDate.AddDate(0, i, 0),
		}
	}
	return installments
}

// redistributeInstallments implements "redistribuir entre os meses": the
// same dates the affected installments already had are kept (so the payoff
// date doesn't move), and balance is split evenly across that same count,
// the last installment absorbing the remainder.
func redistributeInstallments(balance decimal.Decimal, affected []ScenarioTransaction) []GeneratedInstallment {
	n := len(affected)
	amounts := splitEvenly(balance, n)
	installments := make([]GeneratedInstallment, n)
	for i, amount := range amounts {
		installments[i] = GeneratedInstallment{
			Description: fmt.Sprintf("Parcela %d/%d (reajustada)", i+1, n),
			Amount:      amount,
			ProjectedAt: affected[i].ProjectedAt,
		}
	}
	return installments
}
