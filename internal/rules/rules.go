// Package rules implements a small, combinable condition-matching core,
// shared by internal/automation (auto-ignore rules matching on transaction
// text) and internal/recurrences (reconciling a RecurringCommitment's
// expected monthly occurrence against real transactions by category/amount/
// day). It started as internal/automation/matching.go and was generalized
// here so both packages share one matching engine instead of two parallel
// implementations — see .specs/relatorio-financeiro/m0-motor-de-regras.md.
package rules

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

type ConditionField string

const (
	FieldDescription ConditionField = "description"
	FieldCard        ConditionField = "card"
	FieldAccount     ConditionField = "account"
	FieldAmount      ConditionField = "amount"
	FieldDayOfMonth  ConditionField = "day_of_month"
)

type ConditionOperator string

const (
	OperatorContains ConditionOperator = "contains"
	OperatorEquals   ConditionOperator = "equals"
	// OperatorWithinPercent and OperatorNearDay carry their reference value
	// inside Condition.Value as "<reference>:<tolerance>" (see package doc
	// and m0-motor-de-regras.md) because Matches only ever sees the
	// candidate being tested, never a second "expected" value.
	OperatorWithinPercent ConditionOperator = "within_percent"
	OperatorNearDay       ConditionOperator = "near_day"
)

type LogicOperator string

const (
	LogicAnd LogicOperator = "and"
	LogicOr  LogicOperator = "or"
)

// Condition is one field/operator/value test within a rule.
type Condition struct {
	Field    ConditionField
	Operator ConditionOperator
	Value    string
}

// MatchCandidate is the set of fields a Condition can test against.
type MatchCandidate struct {
	Description        *string
	CardNumber         *string
	AccountName        *string
	AccountInstitution *string
	Amount             *decimal.Decimal
	DayOfMonth         *int
}

// normalize mirrors _normalize (strip + casefold). Go's strings.ToLower is a
// practical stand-in for Python's casefold: both make ASCII (and the vast
// majority of real-world) text case-insensitively comparable; casefold's
// extra Unicode special-casing (e.g. German ß) is not worth a dependency for
// matching bank transaction descriptions and account names.
func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func textMatches(candidateValue *string, operator ConditionOperator, value string) bool {
	if candidateValue == nil {
		return false
	}
	nc, nv := normalize(*candidateValue), normalize(value)
	if operator == OperatorEquals {
		return nc == nv
	}
	return strings.Contains(nc, nv)
}

// splitReferenceAndTolerance parses a "<reference>:<tolerance>" Value into
// its two float64 parts. ok is false if the value isn't well-formed, in
// which case the condition never matches (a malformed condition is treated
// as impossible to satisfy, not a panic).
func splitReferenceAndTolerance(value string) (reference, tolerance float64, ok bool) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	reference, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, false
	}
	tolerance, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, false
	}
	return reference, tolerance, true
}

func withinPercentMatches(candidateAmount *decimal.Decimal, value string) bool {
	if candidateAmount == nil {
		return false
	}
	reference, toleranceFraction, ok := splitReferenceAndTolerance(value)
	if !ok || reference == 0 {
		return false
	}
	referenceDecimal := decimal.NewFromFloat(reference)
	diff := candidateAmount.Sub(referenceDecimal).Abs()
	toleranceDecimal := referenceDecimal.Abs().Mul(decimal.NewFromFloat(toleranceFraction / 100))
	return diff.LessThanOrEqual(toleranceDecimal)
}

func nearDayMatches(candidateDay *int, value string) bool {
	if candidateDay == nil {
		return false
	}
	referenceFloat, toleranceFloat, ok := splitReferenceAndTolerance(value)
	if !ok {
		return false
	}
	reference, tolerance := int(referenceFloat), int(toleranceFloat)
	diff := *candidateDay - reference
	if diff < 0 {
		diff = -diff
	}
	// Wrap-around distance (e.g. day 31 vs day 1 is 1 day apart, not 30) so
	// a commitment near a month boundary isn't penalized for the calendar's
	// varying month length.
	wrapped := 31 - diff
	if wrapped < diff {
		diff = wrapped
	}
	return diff <= tolerance
}

func conditionMatches(candidate MatchCandidate, condition Condition) bool {
	switch condition.Field {
	case FieldDescription:
		return textMatches(candidate.Description, condition.Operator, condition.Value)
	case FieldCard:
		return textMatches(candidate.CardNumber, condition.Operator, condition.Value)
	case FieldAmount:
		return withinPercentMatches(candidate.Amount, condition.Value)
	case FieldDayOfMonth:
		return nearDayMatches(candidate.DayOfMonth, condition.Value)
	default: // FieldAccount
		return textMatches(candidate.AccountName, condition.Operator, condition.Value) ||
			textMatches(candidate.AccountInstitution, condition.Operator, condition.Value)
	}
}

// Matches mirrors matches: an empty conditions slice never occurs in
// practice (the schema and the write-side validation both require at least
// one), but "and" over zero conditions is vacuously true and "or" is false,
// matching Python's all()/any() on an empty generator.
func Matches(candidate MatchCandidate, conditions []Condition, logic LogicOperator) bool {
	if logic == LogicAnd {
		for _, c := range conditions {
			if !conditionMatches(candidate, c) {
				return false
			}
		}
		return true
	}
	for _, c := range conditions {
		if conditionMatches(candidate, c) {
			return true
		}
	}
	return false
}
