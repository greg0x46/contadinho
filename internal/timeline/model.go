// Package timeline builds the single financial timeline series every layer
// of the "Relatório Financeiro" (cards, chart, drill-down) consumes — see
// .specs/relatorio-financeiro.md and
// .specs/relatorio-financeiro/m3-historico-e-situacao-atual.md. Nothing
// downstream recomputes totals locally: BuildSeries is the one place that
// merges sources into Entries, so reconciliation across presentation
// layers is guaranteed by construction rather than by convention.
//
// M3 only produces TierRealizado/SourceReal entries (real transactions).
// M4 adds Confirmado/Projetado (recurring commitments, payable plans); M5
// adds Hipotético (standalone scenarios).
package timeline

import (
	"time"

	"github.com/shopspring/decimal"
)

// CertaintyTier is how sure the timeline is that an Entry will happen.
type CertaintyTier string

const (
	TierRealizado  CertaintyTier = "realizado"
	TierConfirmado CertaintyTier = "confirmado"
	TierProjetado  CertaintyTier = "projetado"
	TierHipotetico CertaintyTier = "hipotetico"
)

// SourceKind is which domain concept produced an Entry.
type SourceKind string

const (
	SourceReal        SourceKind = "real"
	SourceRecurring   SourceKind = "recorrente"
	SourcePayablePlan SourceKind = "plano_pagamento"
	SourceScenario    SourceKind = "cenario"
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
	ScenarioID   *string // set only when Source == SourceScenario (from M5 on)
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
