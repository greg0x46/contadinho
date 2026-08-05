package settings

import "context"

// KeyTransactionsPeriodBasis selects which date drives the month a credit
// card transaction is grouped/displayed under: the purchase date
// (PeriodBasisOccurredAt, today's behavior) or the bill's due date once
// known (PeriodBasisPaidAt). Non-credit-card transactions are unaffected —
// there is no lag between "occurred" and "paid" for those.
const KeyTransactionsPeriodBasis = "transactions.period_basis"

const (
	PeriodBasisOccurredAt = "occurred_at"
	PeriodBasisPaidAt     = "paid_at"
)

// GetTransactionsPeriodBasis reads KeyTransactionsPeriodBasis, defaulting to
// PeriodBasisOccurredAt (today's behavior) when the user has never set it.
func GetTransactionsPeriodBasis(ctx context.Context, q Querier) (string, error) {
	value, ok, err := Get(ctx, q, KeyTransactionsPeriodBasis, nil)
	if err != nil {
		return "", err
	}
	if !ok || value == "" {
		return PeriodBasisOccurredAt, nil
	}
	return value, nil
}
