package money_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/money"
)

func strp(s string) *string { return &s }

func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func decp(s string) *decimal.Decimal {
	d := dec(s)
	return &d
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name         string
		movementType *string
		want         money.Classification
	}{
		{"credit is inflow", strp("CREDIT"), money.Inflow},
		{"debit is outflow", strp("DEBIT"), money.Outflow},
		{"unknown value is unclassified", strp("TRANSFER"), money.Unclassified},
		{"nil is unclassified", nil, money.Unclassified},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := money.Classify(tc.movementType); got != tc.want {
				t.Errorf("Classify(%v) = %v, want %v", tc.movementType, got, tc.want)
			}
		})
	}
}

func TestSelectEffectiveMoney(t *testing.T) {
	cases := []struct {
		name                    string
		amountInAccountCurrency *decimal.Decimal
		accountCurrency         *string
		amount                  *decimal.Decimal
		transactionCurrency     *string
		want                    *money.EffectiveMoney
	}{
		{
			name:                    "account currency pair wins when both present",
			amountInAccountCurrency: decp("10.50"),
			accountCurrency:         strp("BRL"),
			amount:                  decp("2.00"),
			transactionCurrency:     strp("USD"),
			want:                    &money.EffectiveMoney{Value: dec("10.50"), CurrencyCode: "BRL", Source: money.AccountCurrency},
		},
		{
			name:                    "falls back to transaction currency when account currency missing",
			amountInAccountCurrency: decp("10.50"),
			accountCurrency:         strp(""),
			amount:                  decp("2.00"),
			transactionCurrency:     strp("USD"),
			want:                    &money.EffectiveMoney{Value: dec("2.00"), CurrencyCode: "USD", Source: money.TransactionCurrency},
		},
		{
			name:                    "falls back when account-currency amount absent",
			amountInAccountCurrency: nil,
			accountCurrency:         strp("BRL"),
			amount:                  decp("2.00"),
			transactionCurrency:     strp("USD"),
			want:                    &money.EffectiveMoney{Value: dec("2.00"), CurrencyCode: "USD", Source: money.TransactionCurrency},
		},
		{
			name: "nil when nothing usable is present",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := money.SelectEffectiveMoney(tc.amountInAccountCurrency, tc.accountCurrency, tc.amount, tc.transactionCurrency)
			if (got == nil) != (tc.want == nil) {
				t.Fatalf("SelectEffectiveMoney() = %v, want %v", got, tc.want)
			}
			if got == nil {
				return
			}
			if !got.Value.Equal(tc.want.Value) || got.CurrencyCode != tc.want.CurrencyCode || got.Source != tc.want.Source {
				t.Errorf("SelectEffectiveMoney() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestEligibility(t *testing.T) {
	brl := &money.EffectiveMoney{Value: dec("10.00"), CurrencyCode: "BRL", Source: money.AccountCurrency}
	zero := &money.EffectiveMoney{Value: dec("0"), CurrencyCode: "BRL", Source: money.AccountCurrency}

	cases := []struct {
		name           string
		classification money.Classification
		providerStatus *string
		money          *money.EffectiveMoney
		inclusionState money.InclusionState
		wantIncluded   bool
		wantReason     money.EligibilityReason
	}{
		{"ignored wins over everything else", money.Outflow, strp("POSTED"), brl, money.Ignored, false, money.ReasonIgnored},
		{"uncategorized transaction is still eligible", money.Outflow, strp("POSTED"), brl, money.Considered, true, ""},
		{"unclassified movement excluded", money.Unclassified, strp("POSTED"), brl, money.Considered, false, money.ReasonUnclassified},
		{"non-posted/pending status excluded", money.Outflow, strp("PROCESSING"), brl, money.Considered, false, money.ReasonIneligibleStatus},
		{"nil status excluded", money.Outflow, nil, brl, money.Considered, false, money.ReasonIneligibleStatus},
		{"missing money excluded", money.Outflow, strp("POSTED"), nil, money.Considered, false, money.ReasonMissingMoneyPair},
		{"zero value excluded", money.Outflow, strp("POSTED"), zero, money.Considered, false, money.ReasonZeroValue},
		{"eligible", money.Outflow, strp("POSTED"), brl, money.Considered, true, ""},
		{"pending status is eligible", money.Inflow, strp("PENDING"), brl, money.Considered, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			included, reason := money.Eligibility(tc.classification, tc.providerStatus, tc.money, tc.inclusionState)
			if included != tc.wantIncluded {
				t.Errorf("included = %v, want %v", included, tc.wantIncluded)
			}
			if tc.wantIncluded {
				if reason != nil {
					t.Errorf("reason = %v, want nil", *reason)
				}
				return
			}
			if reason == nil || *reason != tc.wantReason {
				t.Errorf("reason = %v, want %v", reason, tc.wantReason)
			}
		})
	}
}

func TestCanonicalDecimal(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"1.50", "1.50"},
		{"0", "0"},
		{"-0", "0"},
		{"-10.25", "-10.25"},
		{"100", "100"},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			if got := money.CanonicalDecimal(dec(tc.value)); got != tc.want {
				t.Errorf("CanonicalDecimal(%s) = %s, want %s", tc.value, got, tc.want)
			}
		})
	}
}

func TestPeriodForNoneAndUndated(t *testing.T) {
	p, err := money.PeriodFor(nil, money.GroupNone, "America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "all" || p.Kind != money.PeriodNone {
		t.Errorf("group_by=none: got %+v", p)
	}

	occurred := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	p, err = money.PeriodFor(&occurred, money.GroupDay, "America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind == money.PeriodUndated {
		t.Errorf("dated transaction should not produce undated period")
	}

	p, err = money.PeriodFor(nil, money.GroupDay, "America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "undated" || p.Kind != money.PeriodUndated {
		t.Errorf("nil occurred_at with a real group_by: got %+v", p)
	}
}

func TestPeriodForDayCrossesMidnightInTimezone(t *testing.T) {
	// 2026-03-15 02:00 UTC is 2026-03-14 23:00 in America/Sao_Paulo (UTC-3),
	// so the bucket must reflect the local date, not the UTC one.
	occurred := time.Date(2026, 3, 15, 2, 0, 0, 0, time.UTC)
	p, err := money.PeriodFor(&occurred, money.GroupDay, "America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "day:2026-03-14" {
		t.Errorf("Key = %s, want day:2026-03-14", p.Key)
	}
}

func TestPeriodForWeekStartsMonday(t *testing.T) {
	// 2026-03-18 is a Wednesday; the week should start Monday 2026-03-16.
	occurred := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	p, err := money.PeriodFor(&occurred, money.GroupWeek, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "week:2026-03-16" {
		t.Errorf("Key = %s, want week:2026-03-16", p.Key)
	}
	if p.EndDate.String() != "2026-03-22" {
		t.Errorf("EndDate = %s, want 2026-03-22", p.EndDate)
	}
}

func TestPeriodForMonthHandlesDecemberRollover(t *testing.T) {
	occurred := time.Date(2026, 12, 15, 12, 0, 0, 0, time.UTC)
	p, err := money.PeriodFor(&occurred, money.GroupMonth, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "month:2026-12" {
		t.Errorf("Key = %s, want month:2026-12", p.Key)
	}
	if p.EndDate.String() != "2026-12-31" {
		t.Errorf("EndDate = %s, want 2026-12-31", p.EndDate)
	}
}

func TestPeriodForYear(t *testing.T) {
	occurred := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	p, err := money.PeriodFor(&occurred, money.GroupYear, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "year:2026" {
		t.Errorf("Key = %s, want year:2026", p.Key)
	}
	if p.StartDate.String() != "2026-01-01" || p.EndDate.String() != "2026-12-31" {
		t.Errorf("bounds = %s..%s", p.StartDate, p.EndDate)
	}
}
