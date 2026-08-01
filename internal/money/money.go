// Package money ports the pure decision rules from the Python reference's
// app/transactions/rules.py: classifying a transaction's movement direction,
// picking which of the two amount/currency pairs a transaction carries is the
// "effective" one, and deciding whether a transaction counts toward totals.
// None of this touches the database — it exists so the same rules can be
// unit-tested in isolation and reused by the query engine, debt linking, and
// the sync pipeline alike.
package money

import (
	"github.com/shopspring/decimal"
)

// Classification is the direction of a transaction's movement.
type Classification string

const (
	Inflow       Classification = "inflow"
	Outflow      Classification = "outflow"
	Unclassified Classification = "unclassified"
)

// Classify mirrors rules.classify: only CREDIT/DEBIT movement types are
// meaningful, everything else (including an absent movement type) is
// unclassified.
func Classify(movementType *string) Classification {
	if movementType == nil {
		return Unclassified
	}
	switch *movementType {
	case "CREDIT":
		return Inflow
	case "DEBIT":
		return Outflow
	default:
		return Unclassified
	}
}

// InclusionState is whether a transaction counts toward totals ("considered")
// or has been manually or automatically excluded ("ignored").
type InclusionState string

const (
	Considered InclusionState = "considered"
	Ignored    InclusionState = "ignored"
)

// Source identifies which amount/currency pair an EffectiveMoney came from.
type Source string

const (
	AccountCurrency     Source = "account_currency"
	TransactionCurrency Source = "transaction_currency"
)

// EffectiveMoney is the (value, currency, source) triple a transaction should
// be treated as worth, once the two possible amount/currency pairs a provider
// can supply have been reconciled into one.
type EffectiveMoney struct {
	Value        decimal.Decimal
	CurrencyCode string
	Source       Source
}

// SelectEffectiveMoney mirrors rules.select_effective_money: the
// account-currency amount wins whenever both it and a non-empty account
// currency are present, otherwise the transaction-currency amount is used if
// available, otherwise there is no effective money at all.
func SelectEffectiveMoney(
	amountInAccountCurrency *decimal.Decimal,
	accountCurrency *string,
	amount *decimal.Decimal,
	transactionCurrency *string,
) *EffectiveMoney {
	if amountInAccountCurrency != nil && isTruthyCurrency(accountCurrency) {
		return &EffectiveMoney{Value: *amountInAccountCurrency, CurrencyCode: *accountCurrency, Source: AccountCurrency}
	}
	if amount != nil && isTruthyCurrency(transactionCurrency) {
		return &EffectiveMoney{Value: *amount, CurrencyCode: *transactionCurrency, Source: TransactionCurrency}
	}
	return nil
}

func isTruthyCurrency(s *string) bool {
	return s != nil && *s != ""
}

// CategoryKind is the internal category's role, used for display and
// category management only — it no longer affects totals eligibility.
type CategoryKind string

const (
	Expense  CategoryKind = "expense"
	Income   CategoryKind = "income"
	Transfer CategoryKind = "transfer"
)

// EligibilityReason explains why a transaction was excluded from totals; nil
// (no reason) means it was included.
type EligibilityReason string

const (
	ReasonIgnored          EligibilityReason = "ignored"
	ReasonUnclassified     EligibilityReason = "unclassified"
	ReasonIneligibleStatus EligibilityReason = "ineligible_status"
	ReasonMissingMoneyPair EligibilityReason = "missing_money_pair"
	ReasonZeroValue        EligibilityReason = "zero_value"
)

var eligibleProviderStatuses = map[string]bool{"POSTED": true, "PENDING": true}

// Eligibility mirrors rules.eligibility: the checks run in a fixed order (an
// ignored transaction is reported as "ignored" even if it would also fail a
// later check), because only the first failing reason is ever shown to the
// user. Category assignment plays no part here — categories exist to label a
// transaction, not to gate whether it counts toward totals; only an explicit
// "ignored" inclusion decision excludes a transaction that way.
func Eligibility(
	classification Classification,
	providerStatus *string,
	money *EffectiveMoney,
	inclusionState InclusionState,
) (bool, *EligibilityReason) {
	reason := func(r EligibilityReason) (bool, *EligibilityReason) { return false, &r }

	if inclusionState == Ignored {
		return reason(ReasonIgnored)
	}
	if classification == Unclassified {
		return reason(ReasonUnclassified)
	}
	if providerStatus == nil || !eligibleProviderStatuses[*providerStatus] {
		return reason(ReasonIneligibleStatus)
	}
	if money == nil {
		return reason(ReasonMissingMoneyPair)
	}
	if money.Value.IsZero() {
		return reason(ReasonZeroValue)
	}
	return true, nil
}

// CanonicalDecimal renders value the way the reference API does: fixed-point
// with the value's own scale preserved (decimal.String() trims trailing
// zeros, which would silently change "1.50" to "1.5"), never scientific
// notation, and never a signed zero.
func CanonicalDecimal(value decimal.Decimal) string {
	if value.IsZero() {
		value = value.Abs()
	}
	var places int32
	if exp := value.Exponent(); exp < 0 {
		places = -exp
	}
	return value.StringFixed(places)
}

// GroupBy is the time bucket a transaction query groups by.
type GroupBy string

const (
	GroupNone  GroupBy = "none"
	GroupDay   GroupBy = "day"
	GroupWeek  GroupBy = "week"
	GroupMonth GroupBy = "month"
	GroupYear  GroupBy = "year"
)

// PeriodKind is a Period's bucket kind, which also covers "undated" — a
// possibility GroupBy itself does not need to express, since it only selects
// the bucketing strategy, not a specific bucket's outcome.
type PeriodKind string

const (
	PeriodNone    PeriodKind = "none"
	PeriodDay     PeriodKind = "day"
	PeriodWeek    PeriodKind = "week"
	PeriodMonth   PeriodKind = "month"
	PeriodYear    PeriodKind = "year"
	PeriodUndated PeriodKind = "undated"
)

// Period is the time bucket a transaction falls into for a given GroupBy.
type Period struct {
	Key       string
	Kind      PeriodKind
	StartDate *Date
	EndDate   *Date
}

func (p EligibilityReason) String() string { return string(p) }
