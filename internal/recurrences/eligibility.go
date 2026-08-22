package recurrences

import (
	"github.com/shopspring/decimal"

	"contadinho-go/internal/money"
	"contadinho-go/internal/transactions"
)

// ReconcileIneligibilityReason says why a transaction can't be hand-picked
// for an occurrence. Mirrors payables.LinkIneligibilityReason, including the
// habit of naming the reason rather than returning a bare false — the HTTP
// layer turns it into a 422 the user can act on.
type ReconcileIneligibilityReason string

const (
	ReasonIgnored           ReconcileIneligibilityReason = "ignored"
	ReasonNotOutflow        ReconcileIneligibilityReason = "not_outflow"
	ReasonNotInflow         ReconcileIneligibilityReason = "not_inflow"
	ReasonMissingBRLPair    ReconcileIneligibilityReason = "missing_brl_pair"
	ReasonAlreadyReconciled ReconcileIneligibilityReason = "already_reconciled"
)

// ReconcileEligibility is the verdict for one candidate transaction.
type ReconcileEligibility struct {
	Eligible bool
	Reason   *ReconcileIneligibilityReason
}

// EligibilityForReconciliation decides whether a real transaction may be
// hand-linked to an occurrence of a commitment of this Kind: only BRL
// transactions flowing in the direction the commitment describes (an inflow
// for income, an outflow for an expense), that aren't ignored, and that
// aren't already reconciled to some other occurrence.
//
// Deliberately narrower than payables' equivalent in one way and wider in
// another. Narrower: an ignored transaction is out, because a transaction
// excluded from the totals can't be the thing that carries a commitment's
// money. Wider: unlike CandidatesByMonth, this does NOT require the
// commitment's category or account — reconciling by hand is often exactly
// what a user does *because* the category on the transaction is wrong, and
// refusing the link would leave them with no way to fix it.
func EligibilityForReconciliation(
	kind Kind,
	classification money.Classification,
	inclusionState money.InclusionState,
	eff *money.EffectiveMoney,
	alreadyReconciled bool,
) ReconcileEligibility {
	reason := func(r ReconcileIneligibilityReason) ReconcileEligibility {
		return ReconcileEligibility{Eligible: false, Reason: &r}
	}

	wantClassification := money.Outflow
	notWantReason := ReasonNotOutflow
	if kind == KindIncome {
		wantClassification, notWantReason = money.Inflow, ReasonNotInflow
	}

	if inclusionState == money.Ignored {
		return reason(ReasonIgnored)
	}
	if classification != wantClassification {
		return reason(notWantReason)
	}
	if eff == nil || eff.CurrencyCode != "BRL" {
		return reason(ReasonMissingBRLPair)
	}
	if alreadyReconciled {
		return reason(ReasonAlreadyReconciled)
	}
	return ReconcileEligibility{Eligible: true}
}

// EligibilityForItem is EligibilityForReconciliation for an Item the
// transactions engine already produced, so callers holding one don't
// re-derive classification/effective money from raw columns — the exact
// re-derivation that lets two code paths drift apart.
func EligibilityForItem(kind Kind, item transactions.Item, alreadyReconciled bool) ReconcileEligibility {
	var eff *money.EffectiveMoney
	if item.EffectiveMoney != nil {
		value, err := decimal.NewFromString(item.EffectiveMoney.Value)
		if err != nil {
			r := ReasonMissingBRLPair
			return ReconcileEligibility{Eligible: false, Reason: &r}
		}
		eff = &money.EffectiveMoney{
			Value:        value,
			CurrencyCode: item.EffectiveMoney.CurrencyCode,
			Source:       item.EffectiveMoney.Source,
		}
	}
	return EligibilityForReconciliation(kind, item.Classification, item.Inclusion.State, eff, alreadyReconciled)
}
