package recurrences

import (
	"fmt"
	"sort"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/rules"
	"contadinho-go/internal/transactions"
)

// reconciliationConditions resolves the linked automation rule's stored
// conditions against this specific occurrence. FieldAmount/FieldDayOfMonth
// conditions store only a tolerance (see internal/automation's Write.Validate
// doc comment); here that tolerance is combined with the occurrence's own
// expected amount/day into the "<reference>:<tolerance>" Value internal/rules
// expects. Every other field's condition is already literal and passes
// through unchanged.
func reconciliationConditions(occurrence Occurrence, conditions []rules.Condition) []rules.Condition {
	resolved := make([]rules.Condition, len(conditions))
	for i, c := range conditions {
		switch c.Field {
		case rules.FieldAmount:
			resolved[i] = rules.Condition{
				Field:    rules.FieldAmount,
				Operator: rules.OperatorWithinPercent,
				Value:    fmt.Sprintf("%s:%s", occurrence.ExpectedAmount.String(), c.Value),
			}
		case rules.FieldDayOfMonth:
			resolved[i] = rules.Condition{
				Field:    rules.FieldDayOfMonth,
				Operator: rules.OperatorNearDay,
				Value:    fmt.Sprintf("%d:%s", occurrence.Date.Day(), c.Value),
			}
		default:
			resolved[i] = c
		}
	}
	return resolved
}

func matchCandidateFor(item transactions.Item) (rules.MatchCandidate, bool) {
	if item.OccurredAt == nil || item.EffectiveMoney == nil {
		return rules.MatchCandidate{}, false
	}
	amount, err := decimal.NewFromString(item.EffectiveMoney.Value)
	if err != nil {
		return rules.MatchCandidate{}, false
	}
	amount = amount.Abs()
	day := item.OccurredAt.Day()
	return rules.MatchCandidate{Amount: &amount, DayOfMonth: &day}, true
}

// ResolveOccurrence decides whether occurrence has already been satisfied by
// a real transaction among candidates (expected to already be filtered by
// the caller to the commitment's category/account and the occurrence's
// month via transactions.Query). conditions/logic come from the automation
// rule linked to the commitment via a "reconcile" action
// (internal/automation.ListActiveReconcileTargets) — callers with no linked
// rule should not call this at all, since there's nothing to match against.
// Candidates are tried in date order and the first one to match wins — the
// same "first match wins" rule automation.ApplyToNewTransaction uses.
func ResolveOccurrence(occurrence Occurrence, conditions []rules.Condition, logic rules.LogicOperator, candidates []transactions.Item) (matched *transactions.Item, ok bool) {
	sorted := make([]transactions.Item, len(candidates))
	copy(sorted, candidates)
	sort.SliceStable(sorted, func(i, j int) bool {
		ti, tj := sorted[i].OccurredAt, sorted[j].OccurredAt
		if ti == nil || tj == nil {
			return false
		}
		return ti.Before(*tj)
	})

	resolved := reconciliationConditions(occurrence, conditions)
	for i := range sorted {
		candidate, valid := matchCandidateFor(sorted[i])
		if !valid {
			continue
		}
		if rules.Matches(candidate, resolved, logic) {
			return &sorted[i], true
		}
	}
	return nil, false
}
