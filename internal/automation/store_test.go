package automation_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/categories"
	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
	"contadinho-go/internal/recurrences"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func sampleWrite() automation.Write {
	return automation.Write{
		Name: "Ignorar transferências", IsActive: true, LogicOperator: automation.LogicOr,
		Conditions: []automation.Condition{
			{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "transferencia"},
			{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "pix mesma titularidade"},
		},
		Actions: []automation.ActionWrite{{Type: automation.ActionIgnore}},
	}
}

func TestCreateGetUpdateDelete(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	rule, err := automation.Create(ctx, conn, sampleWrite())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(rule.Conditions) != 2 {
		t.Fatalf("len(Conditions) = %d, want 2", len(rule.Conditions))
	}
	if len(rule.Actions) != 1 || rule.Actions[0].Type != automation.ActionIgnore {
		t.Fatalf("Actions = %+v, want a single ignore action", rule.Actions)
	}

	got, err := automation.Get(ctx, conn, rule.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != rule.Name || len(got.Conditions) != 2 || len(got.Actions) != 1 {
		t.Errorf("Get() = %+v", got)
	}

	updated, err := automation.Update(ctx, conn, rule.ID, automation.Write{
		Name: "Novo nome", IsActive: false, LogicOperator: automation.LogicAnd,
		Conditions: []automation.Condition{{Field: automation.FieldCard, Operator: automation.OperatorEquals, Value: "1234"}},
		Actions:    []automation.ActionWrite{{Type: automation.ActionIgnore}},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Novo nome" || updated.IsActive || len(updated.Conditions) != 1 || len(updated.Actions) != 1 {
		t.Errorf("Update() = %+v", updated)
	}
	if !updated.CreatedAt.Equal(rule.CreatedAt) {
		t.Error("Update should preserve CreatedAt")
	}

	if _, err := automation.SetActive(ctx, conn, rule.ID, true); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	reactivated, err := automation.Get(ctx, conn, rule.ID)
	if err != nil || !reactivated.IsActive {
		t.Errorf("SetActive did not persist: %+v, err=%v", reactivated, err)
	}

	if err := automation.Delete(ctx, conn, rule.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := automation.Get(ctx, conn, rule.ID); !errors.Is(err, automation.ErrNotFound) {
		t.Errorf("Get after delete: err = %v, want ErrNotFound", err)
	}

	var conditionCount int
	conn.QueryRow(`SELECT COUNT(*) FROM automation_rule_conditions WHERE rule_id = ?`, rule.ID).Scan(&conditionCount)
	if conditionCount != 0 {
		t.Errorf("conditions should cascade-delete, got %d remaining", conditionCount)
	}
	var actionCount int
	conn.QueryRow(`SELECT COUNT(*) FROM automation_rule_actions WHERE rule_id = ?`, rule.ID).Scan(&actionCount)
	if actionCount != 0 {
		t.Errorf("actions should cascade-delete, got %d remaining", actionCount)
	}
}

func TestUpdateAndDeleteUnknownRule(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	if _, err := automation.Update(ctx, conn, "unknown", sampleWrite()); !errors.Is(err, automation.ErrNotFound) {
		t.Errorf("Update: err = %v, want ErrNotFound", err)
	}
	if _, err := automation.SetActive(ctx, conn, "unknown", true); !errors.Is(err, automation.ErrNotFound) {
		t.Errorf("SetActive: err = %v, want ErrNotFound", err)
	}
	if err := automation.Delete(ctx, conn, "unknown"); !errors.Is(err, automation.ErrNotFound) {
		t.Errorf("Delete: err = %v, want ErrNotFound", err)
	}
}

func TestListActiveExcludesInactiveRules(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	active := sampleWrite()
	inactive := sampleWrite()
	inactive.Name = "Inativa"
	inactive.IsActive = false

	if _, err := automation.Create(ctx, conn, active); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := automation.Create(ctx, conn, inactive); err != nil {
		t.Fatalf("Create: %v", err)
	}

	all, err := automation.List(ctx, conn)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("len(List()) = %d, want 2", len(all))
	}

	activeOnly, err := automation.ListActive(ctx, conn)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(activeOnly) != 1 || activeOnly[0].Name != "Ignorar transferências" {
		t.Errorf("ListActive() = %+v", activeOnly)
	}
}

func newCommitment(t *testing.T, conn *sql.DB) recurrences.RecurringCommitment {
	t.Helper()
	ctx := context.Background()
	category, err := categories.Create(ctx, conn, "Aluguel", money.Expense, "home", "#ff0000")
	if err != nil {
		t.Fatalf("categories.Create: %v", err)
	}
	amount, _ := decimal.NewFromString("1500.00")
	startDate, err := time.Parse("2006-01-02", "2026-01-01")
	if err != nil {
		t.Fatalf("parse start date: %v", err)
	}
	commitment, err := recurrences.Create(ctx, conn, recurrences.Write{
		Name: "Aluguel", Kind: recurrences.KindExpense, Amount: amount,
		CategoryID: category.ID, Cadence: recurrences.CadenceMonthly, DayOfMonth: 5,
		StartDate: startDate, IsActive: true,
	})
	if err != nil {
		t.Fatalf("recurrences.Create: %v", err)
	}
	return commitment
}

func reconcileWrite(commitmentID string) automation.Write {
	id := commitmentID
	return automation.Write{
		Name: "Concilia aluguel", IsActive: true, LogicOperator: automation.LogicAnd,
		Conditions: []automation.Condition{
			{Field: automation.FieldAmount, Operator: automation.OperatorWithinPercent, Value: "10"},
		},
		Actions: []automation.ActionWrite{{Type: automation.ActionReconcile, RecurringCommitmentID: &id}},
	}
}

func TestListActiveReconcileTargetsKeyedByCommitmentID(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	reconciled := newCommitment(t, conn)
	unreconciled := newCommitment(t, conn)
	inactiveRuleTarget := newCommitment(t, conn)

	reconcileRule, err := automation.Create(ctx, conn, reconcileWrite(reconciled.ID))
	if err != nil {
		t.Fatalf("Create reconcile rule: %v", err)
	}
	inactiveRule, err := automation.Create(ctx, conn, reconcileWrite(inactiveRuleTarget.ID))
	if err != nil {
		t.Fatalf("Create inactive reconcile rule: %v", err)
	}
	if _, err := automation.SetActive(ctx, conn, inactiveRule.ID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if _, err := automation.Create(ctx, conn, sampleWrite()); err != nil {
		t.Fatalf("Create ignore rule: %v", err)
	}

	targets, err := automation.ListActiveReconcileTargets(ctx, conn)
	if err != nil {
		t.Fatalf("ListActiveReconcileTargets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("len(targets) = %d, want 1: %+v", len(targets), targets)
	}
	rule, ok := targets[reconciled.ID]
	if !ok {
		t.Fatalf("expected an entry for commitment %s", reconciled.ID)
	}
	if rule.ID != reconcileRule.ID || len(rule.Conditions) != 1 {
		t.Errorf("targets[%s] = %+v", reconciled.ID, rule)
	}
	if _, ok := targets[unreconciled.ID]; ok {
		t.Error("commitment with no linked rule should not appear")
	}
	if _, ok := targets[inactiveRuleTarget.ID]; ok {
		t.Error("commitment linked only to an inactive rule should not appear")
	}
}

func TestCreateReconcileActionRejectsUnknownCommitment(t *testing.T) {
	conn := newTestDB(t)
	if _, err := automation.Create(context.Background(), conn, reconcileWrite("does-not-exist")); !errors.Is(err, automation.ErrRecurringCommitmentNotFound) {
		t.Errorf("Create: err = %v, want ErrRecurringCommitmentNotFound", err)
	}
}
