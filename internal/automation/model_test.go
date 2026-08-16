package automation_test

import (
	"testing"

	"contadinho-go/internal/automation"
)

func baseWrite() automation.Write {
	return automation.Write{
		Name:          "Ignorar assinaturas",
		IsActive:      true,
		LogicOperator: automation.LogicAnd,
		Conditions: []automation.Condition{
			{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "netflix"},
		},
		Actions: []automation.ActionWrite{{Type: automation.ActionIgnore}},
	}
}

func TestValidateRequiresAtLeastOneCondition(t *testing.T) {
	w := baseWrite()
	w.Conditions = nil
	if err := w.Validate(); err == nil {
		t.Error("expected an error when no condition is set")
	}
}

func TestValidateRejectsMismatchedFieldOperatorPairing(t *testing.T) {
	w := baseWrite()
	w.Conditions = []automation.Condition{
		{Field: automation.FieldDescription, Operator: automation.OperatorWithinPercent, Value: "10"},
	}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: description only supports contains/equals")
	}
}

func TestValidateRejectsMoreThanOneAction(t *testing.T) {
	w := baseWrite()
	w.Actions = []automation.ActionWrite{
		{Type: automation.ActionIgnore},
		{Type: automation.ActionIgnore},
	}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: exactly one action is required")
	}
}

func TestValidateReconcileActionRequiresCommitmentID(t *testing.T) {
	w := baseWrite()
	w.Actions = []automation.ActionWrite{{Type: automation.ActionReconcile}}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: reconcile action requires a recurring_commitment_id")
	}
}

func TestValidateIgnoreActionRejectsCommitmentID(t *testing.T) {
	commitmentID := "commitment-1"
	w := baseWrite()
	w.Actions = []automation.ActionWrite{{Type: automation.ActionIgnore, RecurringCommitmentID: &commitmentID}}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: ignore action must not reference a recurring commitment")
	}
}

func TestValidateRejectsUnknownActionType(t *testing.T) {
	w := baseWrite()
	w.Actions = []automation.ActionWrite{{Type: "archive"}}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: unknown action type")
	}
}

func TestValidateAmountConditionRejectedWithoutReconcileAction(t *testing.T) {
	w := baseWrite()
	w.Conditions = []automation.Condition{
		{Field: automation.FieldAmount, Operator: automation.OperatorWithinPercent, Value: "10"},
	}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: amount condition only valid for a reconcile action")
	}
}

func TestValidateAmountConditionAllowedWithReconcileAction(t *testing.T) {
	commitmentID := "commitment-1"
	w := baseWrite()
	w.Actions = []automation.ActionWrite{{Type: automation.ActionReconcile, RecurringCommitmentID: &commitmentID}}
	w.Conditions = []automation.Condition{
		{Field: automation.FieldAmount, Operator: automation.OperatorWithinPercent, Value: "10"},
		{Field: automation.FieldDayOfMonth, Operator: automation.OperatorNearDay, Value: "3"},
	}
	if err := w.Validate(); err != nil {
		t.Errorf("expected amount/day_of_month conditions to be valid with a reconcile action, got %v", err)
	}
}
