package receivables

import "contadinho-go/internal/money"

// LinkIneligibilityReason mirrors debts.LinkIneligibilityReason.
type LinkIneligibilityReason string

const (
	ReasonIgnored        LinkIneligibilityReason = "ignored"
	ReasonNotInflow      LinkIneligibilityReason = "not_inflow"
	ReasonMissingBRLPair LinkIneligibilityReason = "missing_brl_pair"
	ReasonAlreadyLinked  LinkIneligibilityReason = "already_linked"
)

// LinkEligibility mirrors debts.LinkEligibility.
type LinkEligibility struct {
	Eligible bool
	Reason   *LinkIneligibilityReason
}

// EligibilityForLink mirrors debts.EligibilityForLink, but for the inverse
// direction: only BRL inflows that aren't ignored and aren't already linked
// to some other receivable can be linked — a receivable is settled by money
// coming *in*, the opposite of what pays down a debt.
func EligibilityForLink(classification money.Classification, inclusionState money.InclusionState, eff *money.EffectiveMoney, alreadyLinked bool) LinkEligibility {
	reason := func(r LinkIneligibilityReason) LinkEligibility { return LinkEligibility{Eligible: false, Reason: &r} }

	if inclusionState == money.Ignored {
		return reason(ReasonIgnored)
	}
	if classification != money.Inflow {
		return reason(ReasonNotInflow)
	}
	if eff == nil || eff.CurrencyCode != "BRL" {
		return reason(ReasonMissingBRLPair)
	}
	if alreadyLinked {
		return reason(ReasonAlreadyLinked)
	}
	return LinkEligibility{Eligible: true}
}
