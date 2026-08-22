// Package payables unifies what were formerly the separate internal/debts
// and internal/receivables packages: a debt (money the user owes) and a
// receivable (money owed to the user) are tracked identically — total
// amount, a starting settled-amount snapshot, and settled/remaining amounts
// and status always recomputed from the linked transactions, never stored
// — and differ only in Kind (which also selects the outflow/inflow
// direction a transaction must have to be eligible for linking; see
// eligibility.go).
package payables

import (
	"time"

	"github.com/shopspring/decimal"
)

// Kind discriminates a debt (money the user owes) from a receivable (money
// owed to the user).
type Kind string

const (
	KindDebt       Kind = "debt"
	KindReceivable Kind = "receivable"
)

// Status mirrors payable_status.
type Status string

const (
	StatusOpen    Status = "open"
	StatusSettled Status = "settled"
)

// Payable mirrors the Payable model's persisted fields — everything else
// (settled/remaining amount, status) is derived, never stored; see the
// package doc comment.
type Payable struct {
	ID                    string
	Kind                  Kind
	Name                  string
	TotalAmount           decimal.Decimal
	StartingSettledAmount decimal.Decimal
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Link mirrors PayableTransactionLink's persisted fields. LinkedAmount is a
// snapshot taken when the link was created (shown to the user as
// "linked_amount"); it is deliberately NOT what settled_amount sums — that
// uses each link's live, recomputed amount (linkEffectiveAmount) instead,
// so a later sync correcting the transaction's amount is reflected without
// needing to touch this row.
type Link struct {
	ID            string
	PayableID     string
	TransactionID string
	LinkedAmount  decimal.Decimal
	LinkedAt      time.Time
}

// settledAmount mirrors settled_amount, given the payable's
// starting_settled_amount and each linked transaction's current effective
// amount (from linkEffectiveAmount) — never the links' stored LinkedAmount
// snapshots. Unexported: Summarize is this package's one entry point for
// derived state, so nothing outside assembles these pieces itself.
func settledAmount(startingSettledAmount decimal.Decimal, currentLinkAmounts []decimal.Decimal) decimal.Decimal {
	total := startingSettledAmount
	for _, amount := range currentLinkAmounts {
		total = total.Add(amount)
	}
	return total
}

// remainingAmount mirrors remaining_amount: clamped to [0, totalAmount] so
// an over-payment/over-receipt (or a starting_settled_amount edited after
// the fact) never reports negative or more than 100% remaining.
func remainingAmount(totalAmount, settledAmount decimal.Decimal) decimal.Decimal {
	remaining := totalAmount.Sub(settledAmount)
	if remaining.IsNegative() {
		return decimal.Zero
	}
	if remaining.GreaterThan(totalAmount) {
		return totalAmount
	}
	return remaining
}

// statusFor mirrors payable_status.
func statusFor(remainingAmount decimal.Decimal) Status {
	if remainingAmount.IsZero() {
		return StatusSettled
	}
	return StatusOpen
}
