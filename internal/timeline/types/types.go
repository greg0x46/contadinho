// Package types contains the small set of value types shared by the timeline
// and its projection readers. Keeping them below timeline avoids an import
// cycle: timeline assembles projections, while projections still need to
// label their output with the timeline's certainty/source vocabulary.
package types

// Tier is how certain the application is that an event will happen.
type Tier string

// CertaintyTier is retained as a descriptive alias for callers of the
// original timeline API.
type CertaintyTier = Tier

const (
	TierRealizado  Tier = "realizado"
	TierConfirmado Tier = "confirmado"
	TierProjetado  Tier = "projetado"
	TierHipotetico Tier = "hipotetico"
)

// SourceKind identifies the domain concept that produced an event.
type SourceKind string

const (
	SourceReal        SourceKind = "real"
	SourceInvestment  SourceKind = "investment"
	SourceRecurring   SourceKind = "recorrente"
	SourcePayablePlan SourceKind = "plano_pagamento"
	SourceScenario    SourceKind = "cenario"
)
