// Package automation ports app/automation_rules/matching.py and
// app/automation_rules/service.py: the catalog of user-defined rules, and
// matching+applying them (on a newly-synced transaction, and retroactively
// across every existing one) to act on transactions automatically.
//
// The condition-matching primitives below are aliases of internal/rules —
// see .specs/relatorio-financeiro/m0-motor-de-regras.md. A rule whose only
// action is "ignore" only ever populates the description/card/account
// fields of MatchCandidate and only ever writes contains/equals conditions.
// The amount field (within_percent) and day_of_month field (day_range) are
// only meaningful — and only validated as allowed, see Write.Validate — on a
// rule with a "reconcile" action, whose conditions internal/recurrences uses
// to resolve that commitment's occurrences against real transactions at read
// time. Both carry their own reference value/tolerance directly in
// Condition.Value ("<reference>:<tolerance>" and "<min>:<max>" respectively)
// — independent of the linked commitment's own amount/day_of_month, so the
// same condition applies unchanged across every occurrence.
package automation

import "contadinho-go/internal/rules"

type ConditionField = rules.ConditionField

const (
	FieldDescription = rules.FieldDescription
	FieldCard        = rules.FieldCard
	FieldAccount     = rules.FieldAccount
	FieldAmount      = rules.FieldAmount
	FieldDayOfMonth  = rules.FieldDayOfMonth
)

type ConditionOperator = rules.ConditionOperator

const (
	OperatorContains      = rules.OperatorContains
	OperatorEquals        = rules.OperatorEquals
	OperatorWithinPercent = rules.OperatorWithinPercent
	OperatorDayRange      = rules.OperatorDayRange
)

type LogicOperator = rules.LogicOperator

const (
	LogicAnd = rules.LogicAnd
	LogicOr  = rules.LogicOr
)

// Condition is one field/operator/value test within a Rule.
type Condition = rules.Condition

// MatchCandidate mirrors MatchCandidate: the transaction/account fields
// rules can test against.
type MatchCandidate = rules.MatchCandidate

// Matches mirrors matches, delegating to the shared engine.
func Matches(candidate MatchCandidate, conditions []Condition, logic LogicOperator) bool {
	return rules.Matches(candidate, conditions, logic)
}
