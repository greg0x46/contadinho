package payables_test

import (
	"testing"

	"contadinho-go/internal/money"
	"contadinho-go/internal/payables"
)

func TestEligibilityForLink(t *testing.T) {
	brl := &money.EffectiveMoney{Value: dec(t, "-50.00"), CurrencyCode: "BRL", Source: money.AccountCurrency}
	usd := &money.EffectiveMoney{Value: dec(t, "-50.00"), CurrencyCode: "USD", Source: money.AccountCurrency}

	cases := []struct {
		name           string
		kind           payables.Kind
		classification money.Classification
		inclusion      money.InclusionState
		eff            *money.EffectiveMoney
		alreadyLinked  bool
		wantEligible   bool
		wantReason     payables.LinkIneligibilityReason
	}{
		{"debt: ignored transaction excluded", payables.KindDebt, money.Outflow, money.Ignored, brl, false, false, payables.ReasonIgnored},
		{"debt: inflow excluded", payables.KindDebt, money.Inflow, money.Considered, brl, false, false, payables.ReasonNotOutflow},
		{"debt: unclassified excluded", payables.KindDebt, money.Unclassified, money.Considered, brl, false, false, payables.ReasonNotOutflow},
		{"debt: non-BRL excluded", payables.KindDebt, money.Outflow, money.Considered, usd, false, false, payables.ReasonMissingBRLPair},
		{"debt: missing money excluded", payables.KindDebt, money.Outflow, money.Considered, nil, false, false, payables.ReasonMissingBRLPair},
		{"debt: already linked excluded", payables.KindDebt, money.Outflow, money.Considered, brl, true, false, payables.ReasonAlreadyLinked},
		{"debt: eligible", payables.KindDebt, money.Outflow, money.Considered, brl, false, true, ""},
		{"receivable: ignored transaction excluded", payables.KindReceivable, money.Inflow, money.Ignored, brl, false, false, payables.ReasonIgnored},
		{"receivable: outflow excluded", payables.KindReceivable, money.Outflow, money.Considered, brl, false, false, payables.ReasonNotInflow},
		{"receivable: non-BRL excluded", payables.KindReceivable, money.Inflow, money.Considered, usd, false, false, payables.ReasonMissingBRLPair},
		{"receivable: already linked excluded", payables.KindReceivable, money.Inflow, money.Considered, brl, true, false, payables.ReasonAlreadyLinked},
		{"receivable: eligible", payables.KindReceivable, money.Inflow, money.Considered, brl, false, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := payables.EligibilityForLink(tc.kind, tc.classification, tc.inclusion, tc.eff, tc.alreadyLinked)
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
