package debts_test

import (
	"testing"

	"contadinho-go/internal/debts"
	"contadinho-go/internal/money"
)

func TestEligibilityForLink(t *testing.T) {
	brl := &money.EffectiveMoney{Value: dec(t, "-50.00"), CurrencyCode: "BRL", Source: money.AccountCurrency}
	usd := &money.EffectiveMoney{Value: dec(t, "-50.00"), CurrencyCode: "USD", Source: money.AccountCurrency}

	cases := []struct {
		name           string
		classification money.Classification
		inclusion      money.InclusionState
		eff            *money.EffectiveMoney
		alreadyLinked  bool
		wantEligible   bool
		wantReason     debts.LinkIneligibilityReason
	}{
		{"ignored transaction excluded", money.Outflow, money.Ignored, brl, false, false, debts.ReasonIgnored},
		{"inflow excluded", money.Inflow, money.Considered, brl, false, false, debts.ReasonNotOutflow},
		{"unclassified excluded", money.Unclassified, money.Considered, brl, false, false, debts.ReasonNotOutflow},
		{"non-BRL excluded", money.Outflow, money.Considered, usd, false, false, debts.ReasonMissingBRLPair},
		{"missing money excluded", money.Outflow, money.Considered, nil, false, false, debts.ReasonMissingBRLPair},
		{"already linked excluded", money.Outflow, money.Considered, brl, true, false, debts.ReasonAlreadyLinked},
		{"eligible", money.Outflow, money.Considered, brl, false, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := debts.EligibilityForLink(tc.classification, tc.inclusion, tc.eff, tc.alreadyLinked)
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
