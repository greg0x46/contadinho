// Package receivables mirrors internal/debts for money owed *to* the
// user instead of by them: a Receivable's received/remaining amount and
// status are always recomputed on read from its linked (inflow)
// transactions, never stored.
package receivables

import (
	"time"

	"github.com/shopspring/decimal"
)

// Status mirrors debts.Status.
type Status string

const (
	StatusOpen    Status = "open"
	StatusSettled Status = "settled"
)

// Receivable mirrors the Receivable model's persisted fields — everything
// else (received/remaining amount, status) is derived, never stored.
type Receivable struct {
	ID                     string
	Name                   string
	TotalAmount            decimal.Decimal
	StartingReceivedAmount decimal.Decimal
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// Link mirrors debts.Link: LinkedAmount is a snapshot taken when the link
// was created, but ReceivedAmount uses each link's live, recomputed amount
// (LinkEffectiveAmount) instead, so a later sync correcting the
// transaction's amount is reflected without needing to touch this row.
type Link struct {
	ID            string
	ReceivableID  string
	TransactionID string
	LinkedAmount  decimal.Decimal
	LinkedAt      time.Time
}

// ReceivedAmount mirrors debts.PaidAmount, given the receivable's
// starting_received_amount and each linked transaction's current effective
// amount (from LinkEffectiveAmount) — never the links' stored LinkedAmount
// snapshots.
func ReceivedAmount(startingReceivedAmount decimal.Decimal, currentLinkAmounts []decimal.Decimal) decimal.Decimal {
	total := startingReceivedAmount
	for _, amount := range currentLinkAmounts {
		total = total.Add(amount)
	}
	return total
}

// RemainingAmount mirrors debts.RemainingAmount: clamped to [0,
// totalAmount] so an over-receipt (or a starting_received_amount edited
// after the fact) never reports negative or more than 100% remaining.
func RemainingAmount(totalAmount, receivedAmount decimal.Decimal) decimal.Decimal {
	remaining := totalAmount.Sub(receivedAmount)
	if remaining.IsNegative() {
		return decimal.Zero
	}
	if remaining.GreaterThan(totalAmount) {
		return totalAmount
	}
	return remaining
}

// StatusFor mirrors debts.StatusFor.
func StatusFor(remainingAmount decimal.Decimal) Status {
	if remainingAmount.IsZero() {
		return StatusSettled
	}
	return StatusOpen
}
