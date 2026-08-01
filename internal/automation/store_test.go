package automation_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/db"
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

	got, err := automation.Get(ctx, conn, rule.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != rule.Name || len(got.Conditions) != 2 {
		t.Errorf("Get() = %+v", got)
	}

	updated, err := automation.Update(ctx, conn, rule.ID, automation.Write{
		Name: "Novo nome", IsActive: false, LogicOperator: automation.LogicAnd,
		Conditions: []automation.Condition{{Field: automation.FieldCard, Operator: automation.OperatorEquals, Value: "1234"}},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Novo nome" || updated.IsActive || len(updated.Conditions) != 1 {
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
