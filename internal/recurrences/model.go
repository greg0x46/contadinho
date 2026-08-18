// Package recurrences models a manually-registered recurring cash flow
// (salary, rent, subscriptions) — a compromisso the user knows about ahead
// of time that Pluggy has no way to report before it happens. It's the
// "Confirmado" tier of internal/timeline's projection (see
// .specs/relatorio-financeiro/m1-recorrencias.md): occurrences are computed
// purely from the commitment's own scheduling fields, and reconciled against
// real transactions at read time via internal/rules — never by storing a
// link, mirroring the isolation already used by internal/scenarios'
// scenario_transactions vs financial_transactions. A commitment carries no
// matching criteria of its own: reconciliation conditions live exclusively
// on an internal/automation Rule with a "reconcile" action targeting this
// commitment (internal/automation.ListActiveReconcileTargets) — a
// commitment with no such rule linked simply never reconciles, which is how
// a recurring commitment can exist independent of any automation.
package recurrences

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Kind is the direction of a commitment's cash flow.
type Kind string

const (
	KindIncome  Kind = "income"
	KindExpense Kind = "expense"
)

// Cadence is how often a commitment recurs.
type Cadence string

const (
	CadenceMonthly Cadence = "monthly"
	CadenceAnnual  Cadence = "annual"
)

// RecurringCommitment mirrors the recurring_commitments table.
type RecurringCommitment struct {
	ID          string
	Name        string
	Kind        Kind
	Amount      decimal.Decimal // magnitude, always positive
	CategoryID  string
	AccountID   *string
	Cadence     Cadence
	DayOfMonth  int  // 1-31; clamped to the last day of short months when generating occurrences. Purely a scheduling anchor — never implies a reconciliation condition on its own.
	MonthOfYear *int // required iff Cadence == CadenceAnnual, ignored otherwise
	StartDate   time.Time
	EndDate     *time.Time
	IsActive    bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate reports the first structural problem found, or nil if the
// commitment is well-formed. Callers (the HTTP layer) are expected to
// surface this as a 422, not a panic.
func (c RecurringCommitment) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.Kind != KindIncome && c.Kind != KindExpense {
		return fmt.Errorf("kind must be %q or %q", KindIncome, KindExpense)
	}
	if !c.Amount.IsPositive() {
		return fmt.Errorf("amount must be positive")
	}
	if c.CategoryID == "" {
		return fmt.Errorf("category_id is required")
	}
	if c.Cadence != CadenceMonthly && c.Cadence != CadenceAnnual {
		return fmt.Errorf("cadence must be %q or %q", CadenceMonthly, CadenceAnnual)
	}
	if c.DayOfMonth < 1 || c.DayOfMonth > 31 {
		return fmt.Errorf("day_of_month must be between 1 and 31")
	}
	if c.Cadence == CadenceAnnual {
		if c.MonthOfYear == nil || *c.MonthOfYear < 1 || *c.MonthOfYear > 12 {
			return fmt.Errorf("month_of_year is required and must be between 1 and 12 for an annual cadence")
		}
	}
	if c.EndDate != nil && c.EndDate.Before(c.StartDate) {
		return fmt.Errorf("end_date must not be before start_date")
	}
	return nil
}
