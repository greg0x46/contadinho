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

func TestValidateRequiresAtLeastOneAction(t *testing.T) {
	w := baseWrite()
	w.Actions = nil
	if err := w.Validate(); err == nil {
		t.Error("expected an error: at least one action is required")
	}
}

func TestValidateAllowsMultipleActions(t *testing.T) {
	scenarioID := "scenario-1"
	categoryID := "category-1"
	w := baseWrite()
	w.Actions = []automation.ActionWrite{
		{Type: automation.ActionReconcile, ScenarioID: &scenarioID},
		{Type: automation.ActionSetCategory, CategoryID: &categoryID},
	}
	if err := w.Validate(); err != nil {
		t.Errorf("expected a rule to allow combining reconcile and set_category actions, got %v", err)
	}
}

func TestValidateSetCategoryActionRequiresCategoryID(t *testing.T) {
	w := baseWrite()
	w.Actions = []automation.ActionWrite{{Type: automation.ActionSetCategory}}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: set_category action requires a category_id")
	}
}

func TestValidateIgnoreActionRejectsCategoryID(t *testing.T) {
	categoryID := "category-1"
	w := baseWrite()
	w.Actions = []automation.ActionWrite{{Type: automation.ActionIgnore, CategoryID: &categoryID}}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: ignore action must not reference a category")
	}
}

func TestValidateReconcileActionRequiresScenarioID(t *testing.T) {
	w := baseWrite()
	w.Actions = []automation.ActionWrite{{Type: automation.ActionReconcile}}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: reconcile action requires a scenario_id")
	}
}

func TestValidateIgnoreActionRejectsScenarioID(t *testing.T) {
	scenarioID := "scenario-1"
	w := baseWrite()
	w.Actions = []automation.ActionWrite{{Type: automation.ActionIgnore, ScenarioID: &scenarioID}}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: ignore action must not reference a recurring scenario")
	}
}

func TestValidateRejectsUnknownActionType(t *testing.T) {
	w := baseWrite()
	w.Actions = []automation.ActionWrite{{Type: "archive"}}
	if err := w.Validate(); err == nil {
		t.Error("expected an error: unknown action type")
	}
}

func TestValidateAmountConditionAllowedWithoutReconcileAction(t *testing.T) {
	w := baseWrite()
	w.Conditions = []automation.Condition{
		{Field: automation.FieldAmount, Operator: automation.OperatorWithinPercent, Value: "10"},
	}
	if err := w.Validate(); err != nil {
		t.Errorf("expected amount condition to be valid without a reconcile action, got %v", err)
	}
}

func TestValidateAmountConditionAllowedWithReconcileAction(t *testing.T) {
	scenarioID := "scenario-1"
	w := baseWrite()
	w.Actions = []automation.ActionWrite{{Type: automation.ActionReconcile, ScenarioID: &scenarioID}}
	w.Conditions = []automation.Condition{
		{Field: automation.FieldAmount, Operator: automation.OperatorWithinPercent, Value: "10"},
		{Field: automation.FieldDayOfMonth, Operator: automation.OperatorDayRange, Value: "1:5"},
	}
	if err := w.Validate(); err != nil {
		t.Errorf("expected amount/day_of_month conditions to be valid with a reconcile action, got %v", err)
	}
}
