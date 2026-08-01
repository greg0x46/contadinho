// Package scenarios implements hypothetical, user-authored projections
// layered on top of real data — see
// .specs/plano-pagamento-e-cenarios-projecao.md. A Scenario groups a set of
// ScenarioTransaction "planned installments"; in this v1 the only kind is
// "debt_plan", a payment plan attached to a debts.Debt. Nothing here changes
// how internal/debts computes paid/remaining amounts — a scenario is a
// read-layer projection, never a source of real totals, unless a caller
// explicitly opts in (see the transactions package's scenario_id union).
package scenarios

import (
	"time"

	"github.com/shopspring/decimal"
)

// Kind is the scenario's flavor. Only KindDebtPlan is implemented in this
// v1; "what_if" (a free-standing scenario with no debt_id) is left as a
// possible future kind, per the spec's non-goals.
type Kind string

const KindDebtPlan Kind = "debt_plan"

// Scenario mirrors the scenarios table.
type Scenario struct {
	ID        string
	Kind      Kind
	Name      string
	DebtID    *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ScenarioTransaction mirrors the scenario_transactions table: a single
// planned/projected installment, with a planned Amount and ProjectedAt date
// — never a real financial_transactions row.
type ScenarioTransaction struct {
	ID          string
	ScenarioID  string
	Description string
	Amount      decimal.Decimal
	ProjectedAt time.Time
	Category    *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Status is a ScenarioTransaction's recomputed-on-read state — never
// persisted, mirroring debts.Status's pattern.
type Status string

const (
	StatusAtrasada         Status = "atrasada"
	StatusProjetada        Status = "projetada"
	StatusPagaParcialmente Status = "paga_parcialmente"
	StatusPaga             Status = "paga"
	StatusPagaAMais        Status = "paga_a_mais"
)

// Status computes the installment's status from today's date and
// realizedTotal (the sum of scenario_transaction_realizations.allocated_amount
// allocated to it) — both supplied by the caller, never read here, so this
// stays a pure function callable from tests without a database.
func (s ScenarioTransaction) Status(today time.Time, realizedTotal decimal.Decimal) Status {
	switch {
	case realizedTotal.IsZero() && today.After(s.ProjectedAt):
		return StatusAtrasada
	case realizedTotal.IsZero():
		return StatusProjetada
	case realizedTotal.LessThan(s.Amount):
		return StatusPagaParcialmente
	case realizedTotal.Equal(s.Amount):
		return StatusPaga
	default: // realizedTotal > s.Amount
		return StatusPagaAMais
	}
}
