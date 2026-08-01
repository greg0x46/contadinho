// Package debts ports app/debts/eligibility.py and app/debts/service.py
// (feature 007): debt CRUD, and linking a debt to the transaction(s) that
// paid it down. Every amount beyond a link's own snapshot (paid_amount,
// remaining_amount, status, and even a link's "current" amount) is computed
// on read from the linked transactions' live effective money — never
// persisted — so a later sync correcting a transaction's amount is
// reflected automatically instead of leaving stale totals.
package debts

import "contadinho-go/internal/money"

// LinkIneligibilityReason mirrors LinkIneligibilityReason.
type LinkIneligibilityReason string

const (
	ReasonIgnored        LinkIneligibilityReason = "ignored"
	ReasonNotOutflow     LinkIneligibilityReason = "not_outflow"
	ReasonMissingBRLPair LinkIneligibilityReason = "missing_brl_pair"
	ReasonAlreadyLinked  LinkIneligibilityReason = "already_linked"
)

// LinkEligibility mirrors LinkEligibility.
type LinkEligibility struct {
	Eligible bool
	Reason   *LinkIneligibilityReason
}

// EligibilityForLink mirrors eligibility_for_link: only BRL outflows that
// aren't ignored and aren't already linked to some other debt can be linked
// — debts are a BRL-only concept in this app (per the reference), so a
// transaction without a BRL effective-money pair can never pay one down.
func EligibilityForLink(classification money.Classification, inclusionState money.InclusionState, eff *money.EffectiveMoney, alreadyLinked bool) LinkEligibility {
	reason := func(r LinkIneligibilityReason) LinkEligibility { return LinkEligibility{Eligible: false, Reason: &r} }

	if inclusionState == money.Ignored {
		return reason(ReasonIgnored)
	}
	if classification != money.Outflow {
		return reason(ReasonNotOutflow)
	}
	if eff == nil || eff.CurrencyCode != "BRL" {
		return reason(ReasonMissingBRLPair)
	}
	if alreadyLinked {
		return reason(ReasonAlreadyLinked)
	}
	return LinkEligibility{Eligible: true}
}
