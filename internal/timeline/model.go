// Package timeline builds the single financial timeline series every layer
// of the "Relatório Financeiro" (cards, chart, drill-down) consumes — see
// .specs/contextos/relatorio-financeiro/reference.md and
// .specs/motores-de-dominio.md section 6. Nothing downstream recomputes
// totals locally: BuildSeries is the one place that merges sources into
// Entries, so reconciliation across presentation layers is guaranteed by
// construction rather than by convention.
//
// The three sources and the tiers they produce:
//
//   - real transactions — Realizado, or Confirmado for a credit-card
//     purchase re-dated to its bill's due date;
//   - scenarios — Confirmado for a payable plan's installment (a real
//     debt/receivable with a planned date), Hipotético for a standalone
//     "what if" the caller explicitly selected;
//   - recurring commitments — Projetado, and only for occurrences that no
//     real transaction has reconciled; a reconciled one is already in the
//     series as its real transaction.
//
// This package knows nothing of payables: a plan's link to one stays
// encapsulated in scenarios (Scenario.PayableID/Kind).
package timeline

import (
	"time"

	"github.com/shopspring/decimal"

	timelinetypes "contadinho-go/internal/timeline/types"
)

// These aliases keep the public timeline API stable while allowing the
// projections package to use the same vocabulary without importing this
// package back and creating a cycle.
type Tier = timelinetypes.Tier
type CertaintyTier = timelinetypes.CertaintyTier

const (
	TierRealizado  = timelinetypes.TierRealizado
	TierConfirmado = timelinetypes.TierConfirmado
	TierProjetado  = timelinetypes.TierProjetado
	TierHipotetico = timelinetypes.TierHipotetico
)

type SourceKind = timelinetypes.SourceKind

const (
	SourceReal        = timelinetypes.SourceReal
	SourceRecurring   = timelinetypes.SourceRecurring
	SourcePayablePlan = timelinetypes.SourcePayablePlan
	SourceScenario    = timelinetypes.SourceScenario
)

// Entry is one atomic cash-flow item — the same shape consumed by cards,
// chart, and drill-down alike, never a second parallel representation.
type Entry struct {
	Date         time.Time
	Description  string
	Amount       decimal.Decimal // signed: positive = inflow, negative = outflow
	CategoryID   *string         // nil means "Sem categoria" — never dropped from totals
	CategoryName string
	Tier         CertaintyTier
	Source       SourceKind
	SourceRefID  string
	EventKey     string  // stable identity for projected events; real events use transaction:<id>
	ScenarioID   *string // set for every projected Scenario event
}

// DayPoint is the running balance at the end of one calendar day.
type DayPoint struct {
	Date       time.Time
	Balance    decimal.Decimal
	Inflow     decimal.Decimal
	Outflow    decimal.Decimal
	LowestTier CertaintyTier
}

// Series is BuildSeries' one output — every presentation layer reads from
// this, never recomputing totals independently.
type Series struct {
	Points          []DayPoint
	Entries         []Entry
	StartingBalance decimal.Decimal
	LowestBalance   DayPoint   // meaningful from M4 on; M3 has no future sources to dip below the starting point
	FirstNegative   *time.Time // nil in M3 unless real transactions already carry the balance negative
}
