package transactions

import (
	"testing"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/money"
)

func TestInvestmentAllocationSeparatesReportingFromCash(t *testing.T) {
	for _, tc := range []struct {
		name, allocation, wantReported, wantTransfer string
		classification                               money.Classification
		ignored                                      bool
	}{
		{"deposit", "1000", "0", "1000", money.Outflow, false},
		{"partial deposit", "900", "100", "900", money.Outflow, false},
		{"withdrawal", "1000", "0", "1000", money.Inflow, false},
		{"partial withdrawal", "900", "100", "900", money.Inflow, false},
		{"provider correction", "1200", "0", "1000", money.Outflow, false},
		{"ignored keeps precedence", "1000", "0", "1000", money.Outflow, true},
		{"unlinked", "0", "1000", "0", money.Outflow, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := view{classification: tc.classification, included: true,
				effective: &money.EffectiveMoney{Value: decimal.RequireFromString("1000"), CurrencyCode: "BRL"}}
			if tc.ignored {
				reason := money.ReasonIgnored
				v.included = false
				v.reason = &reason
			}
			v.applyInvestmentTransfer(decimal.RequireFromString(tc.allocation))
			item := toItem(v)
			if item.EffectiveMoney.Value != "1000" || item.InvestmentTransferAmount != tc.wantTransfer || item.ReportableAmount == nil || *item.ReportableAmount != tc.wantReported {
				t.Fatalf("unexpected item: %+v", item)
			}
			if tc.ignored {
				if item.TotalsEligibility.MovesCash() || item.TotalsEligibility.Reason == nil || *item.TotalsEligibility.Reason != money.ReasonIgnored {
					t.Fatal("ignored precedence lost")
				}
			} else if !item.TotalsEligibility.MovesCash() {
				t.Fatal("allocation removed the bank cash movement")
			}
			if tc.wantReported == "0" {
				if len(buildOverallTotals([]view{v})) != 0 {
					t.Fatal("fully transferred/ignored item counted in totals")
				}
			} else {
				totals := buildOverallTotals([]view{v})
				if len(totals) != 1 {
					t.Fatalf("totals = %+v", totals)
				}
				actual := totals[0].Outflow
				if tc.classification == money.Inflow {
					actual = totals[0].Inflow
				}
				if actual != tc.wantReported {
					t.Fatalf("report = %s, want %s", actual, tc.wantReported)
				}
			}
		})
	}
}
