package debts

import (
	"time"

	"github.com/shopspring/decimal"
)

// Status mirrors DebtStatus.
type Status string

const (
	StatusOpen    Status = "open"
	StatusSettled Status = "settled"
)

// Debt mirrors the Debt model's persisted fields — everything else
// (paid/remaining amount, status) is derived, never stored; see the
// package doc comment.
type Debt struct {
	ID                 string
	Name               string
	TotalAmount        decimal.Decimal
	StartingPaidAmount decimal.Decimal
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Link mirrors DebtTransactionLink's persisted fields. LinkedAmount is a
// snapshot taken when the link was created (shown to the user as
// "linked_amount"); it is deliberately NOT what paid_amount sums — that
// uses each link's live, recomputed amount (LinkEffectiveAmount) instead,
// so a later sync correcting the transaction's amount is reflected without
// needing to touch this row.
type Link struct {
	ID            string
	DebtID        string
	TransactionID string
	LinkedAmount  decimal.Decimal
	LinkedAt      time.Time
}

// PaidAmount mirrors paid_amount, given the debt's starting_paid_amount and
// each linked transaction's current effective amount (from
// LinkEffectiveAmount) — never the links' stored LinkedAmount snapshots.
func PaidAmount(startingPaidAmount decimal.Decimal, currentLinkAmounts []decimal.Decimal) decimal.Decimal {
	total := startingPaidAmount
	for _, amount := range currentLinkAmounts {
		total = total.Add(amount)
	}
	return total
}

// RemainingAmount mirrors remaining_amount: clamped to [0, totalAmount] so
// an over-payment (or a starting_paid_amount edited after the fact) never
// reports negative or more than 100% remaining.
func RemainingAmount(totalAmount, paidAmount decimal.Decimal) decimal.Decimal {
	remaining := totalAmount.Sub(paidAmount)
	if remaining.IsNegative() {
		return decimal.Zero
	}
	if remaining.GreaterThan(totalAmount) {
		return totalAmount
	}
	return remaining
}

// StatusFor mirrors debt_status.
func StatusFor(remainingAmount decimal.Decimal) Status {
	if remainingAmount.IsZero() {
		return StatusSettled
	}
	return StatusOpen
}
