// Package automation ports app/automation_rules/matching.py and
// app/automation_rules/service.py: the catalog of user-defined rules, and
// matching+applying them (on a newly-synced transaction, and retroactively
// across every existing one) to ignore transactions automatically.
package automation

import "strings"

type ConditionField string

const (
	FieldDescription ConditionField = "description"
	FieldCard        ConditionField = "card"
	FieldAccount     ConditionField = "account"
)

type ConditionOperator string

const (
	OperatorContains ConditionOperator = "contains"
	OperatorEquals   ConditionOperator = "equals"
)

type LogicOperator string

const (
	LogicAnd LogicOperator = "and"
	LogicOr  LogicOperator = "or"
)

// Condition is one field/operator/value test within a Rule.
type Condition struct {
	Field    ConditionField
	Operator ConditionOperator
	Value    string
}

// MatchCandidate mirrors MatchCandidate: the transaction/account fields
// rules can test against.
type MatchCandidate struct {
	Description        *string
	CardNumber         *string
	AccountName        *string
	AccountInstitution *string
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

func conditionMatches(candidate MatchCandidate, condition Condition) bool {
	switch condition.Field {
	case FieldDescription:
		return textMatches(candidate.Description, condition.Operator, condition.Value)
	case FieldCard:
		return textMatches(candidate.CardNumber, condition.Operator, condition.Value)
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
