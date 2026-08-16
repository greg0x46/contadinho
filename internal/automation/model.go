package automation

import (
	"fmt"
	"strings"
)

// Validate checks a Write for structural correctness: exactly one action
// (the table shape supports more later, application logic pins it to
// exactly one for now — see automation_rule_actions' migration comment),
// its target reference matching its type, and every condition's
// field/operator pairing (amount/day_of_month only allowed when the rule
// has a reconcile action, since those conditions are meaningless without a
// commitment supplying the reference amount/day to inject at resolve time).
func (w Write) Validate() error {
	if strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if w.LogicOperator != LogicAnd && w.LogicOperator != LogicOr {
		return fmt.Errorf("unknown logic operator %q", w.LogicOperator)
	}
	if len(w.Conditions) == 0 {
		return fmt.Errorf("at least one condition is required")
	}
	if len(w.Actions) != 1 {
		return fmt.Errorf("exactly one action is required")
	}

	action := w.Actions[0]
	switch action.Type {
	case ActionIgnore:
		if action.RecurringCommitmentID != nil {
			return fmt.Errorf("ignore action must not reference a recurring commitment")
		}
	case ActionReconcile:
		if action.RecurringCommitmentID == nil || strings.TrimSpace(*action.RecurringCommitmentID) == "" {
			return fmt.Errorf("reconcile action requires a recurring_commitment_id")
		}
	default:
		return fmt.Errorf("unknown action type %q", action.Type)
	}

	allowReconcileFields := action.Type == ActionReconcile
	for _, c := range w.Conditions {
		if err := validateCondition(c, allowReconcileFields); err != nil {
			return err
		}
	}
	return nil
}

func validateCondition(c Condition, allowReconcileFields bool) error {
	switch c.Field {
	case FieldDescription, FieldCard, FieldAccount:
		if c.Operator != OperatorContains && c.Operator != OperatorEquals {
			return fmt.Errorf("field %q only accepts contains/equals, got %q", c.Field, c.Operator)
		}
	case FieldAmount:
		if !allowReconcileFields {
			return fmt.Errorf("field %q is only valid for a reconcile action", FieldAmount)
		}
		if c.Operator != OperatorWithinPercent {
			return fmt.Errorf("field %q only accepts within_percent, got %q", c.Field, c.Operator)
		}
	case FieldDayOfMonth:
		if !allowReconcileFields {
			return fmt.Errorf("field %q is only valid for a reconcile action", FieldDayOfMonth)
		}
		if c.Operator != OperatorNearDay {
			return fmt.Errorf("field %q only accepts near_day, got %q", c.Field, c.Operator)
		}
	default:
		return fmt.Errorf("unknown condition field %q", c.Field)
	}
	if strings.TrimSpace(c.Value) == "" {
		return fmt.Errorf("condition value is required")
	}
	return nil
}
