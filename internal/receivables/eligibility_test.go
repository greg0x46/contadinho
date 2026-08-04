package receivables_test

import (
	"testing"

	"contadinho-go/internal/money"
	"contadinho-go/internal/receivables"
)

func TestEligibilityForLink(t *testing.T) {
	brl := &money.EffectiveMoney{Value: dec(t, "50.00"), CurrencyCode: "BRL", Source: money.AccountCurrency}
	usd := &money.EffectiveMoney{Value: dec(t, "50.00"), CurrencyCode: "USD", Source: money.AccountCurrency}

	cases := []struct {
		name           string
		classification money.Classification
		inclusion      money.InclusionState
		eff            *money.EffectiveMoney
		alreadyLinked  bool
		wantEligible   bool
		wantReason     receivables.LinkIneligibilityReason
	}{
		{"ignored transaction excluded", money.Inflow, money.Ignored, brl, false, false, receivables.ReasonIgnored},
		{"outflow excluded", money.Outflow, money.Considered, brl, false, false, receivables.ReasonNotInflow},
		{"unclassified excluded", money.Unclassified, money.Considered, brl, false, false, receivables.ReasonNotInflow},
		{"non-BRL excluded", money.Inflow, money.Considered, usd, false, false, receivables.ReasonMissingBRLPair},
		{"missing money excluded", money.Inflow, money.Considered, nil, false, false, receivables.ReasonMissingBRLPair},
		{"already linked excluded", money.Inflow, money.Considered, brl, true, false, receivables.ReasonAlreadyLinked},
		{"eligible", money.Inflow, money.Considered, brl, false, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := receivables.EligibilityForLink(tc.classification, tc.inclusion, tc.eff, tc.alreadyLinked)
			if got.Eligible != tc.wantEligible {
				t.Errorf("Eligible = %v, want %v", got.Eligible, tc.wantEligible)
			}
			if tc.wantEligible {
				if got.Reason != nil {
					t.Errorf("Reason = %v, want nil", *got.Reason)
				}
				return
			}
			if got.Reason == nil || *got.Reason != tc.wantReason {
				t.Errorf("Reason = %v, want %v", got.Reason, tc.wantReason)
			}
		})
	}
}
