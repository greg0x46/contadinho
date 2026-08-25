package recurrences_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

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

func newCategory(t *testing.T, conn *sql.DB) categories.Category {
	t.Helper()
	c, err := categories.Create(context.Background(), conn, "Aluguel", money.Expense, "home", "#ff0000")
	if err != nil {
		t.Fatalf("categories.Create: %v", err)
	}
	return c
}

func testWrite(t *testing.T, categoryID string) recurrences.Write {
	return recurrences.Write{
		Name: "Aluguel", Kind: recurrences.KindExpense, Amount: dec(t, "1500.00"),
		CategoryID: categoryID,
		Cadence:    recurrences.CadenceMonthly, DayOfMonth: 5,
		StartDate: date(t, "2026-01-01"), IsActive: true,
	}
}

func TestCreateAndGetRoundTrips(t *testing.T) {
	conn := newTestDB(t)
	category := newCategory(t, conn)

	created, err := recurrences.Create(context.Background(), conn, testWrite(t, category.ID))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	fetched, err := recurrences.Get(context.Background(), conn, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.Name != "Aluguel" || fetched.CategoryID != category.ID || !fetched.Amount.Equal(dec(t, "1500.00")) {
		t.Errorf("unexpected round-tripped commitment: %+v", fetched)
	}
	if fetched.DayOfMonth != 5 || fetched.Cadence != recurrences.CadenceMonthly {
		t.Errorf("unexpected cadence/day: %+v", fetched)
	}
}

func TestUpdateReplacesFields(t *testing.T) {
	conn := newTestDB(t)
	category := newCategory(t, conn)
	created, err := recurrences.Create(context.Background(), conn, testWrite(t, category.ID))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	write := testWrite(t, category.ID)
	write.Amount = dec(t, "1600.00")
	write.DayOfMonth = 10
	updated, err := recurrences.Update(context.Background(), conn, created.ID, write)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.Amount.Equal(dec(t, "1600.00")) || updated.DayOfMonth != 10 {
		t.Errorf("update did not apply: %+v", updated)
	}
}

func TestUpdateUnknownIDReturnsNotFound(t *testing.T) {
	conn := newTestDB(t)
	category := newCategory(t, conn)
	_, err := recurrences.Update(context.Background(), conn, "missing", testWrite(t, category.ID))
	if !errors.Is(err, recurrences.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// A recurring Scenario with no schedule row is the half-formed state the
// projector refuses to project. Update must refuse it too, and refuse it
// before committing: without the schedule's own RowsAffected check the name
// change lands, the read-back then fails, and the caller is told 404 about a
// write that already happened.
func TestUpdateWithoutScheduleRowIsRejectedWithoutWriting(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	category := newCategory(t, conn)
	created, err := recurrences.Create(ctx, conn, testWrite(t, category.ID))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM scenario_recurring_schedules WHERE scenario_id = ?`, created.ID); err != nil {
		t.Fatalf("drop schedule: %v", err)
	}

	write := testWrite(t, category.ID)
	write.Name = "Nome novo"
	if _, err := recurrences.Update(ctx, conn, created.ID, write); !errors.Is(err, recurrences.ErrNotFound) {
		t.Fatalf("Update = %v, want ErrNotFound", err)
	}
	var name string
	if err := conn.QueryRowContext(ctx, `SELECT name FROM scenarios WHERE id = ?`, created.ID).Scan(&name); err != nil {
		t.Fatalf("read back scenario: %v", err)
	}
	if name != "Aluguel" {
		t.Errorf("scenario name = %q, want the rejected update to have rolled back to %q", name, "Aluguel")
	}
}

func TestSetActiveTogglesWithoutChangingOtherFields(t *testing.T) {
	conn := newTestDB(t)
	category := newCategory(t, conn)
	created, err := recurrences.Create(context.Background(), conn, testWrite(t, category.ID))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	paused, err := recurrences.SetActive(context.Background(), conn, created.ID, false)
	if err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if paused.IsActive {
		t.Error("expected commitment to be paused")
	}
	if paused.Amount.Cmp(created.Amount) != 0 {
		t.Error("SetActive must not change other fields")
	}
}

func TestDeleteRemovesCommitment(t *testing.T) {
	conn := newTestDB(t)
	category := newCategory(t, conn)
	created, err := recurrences.Create(context.Background(), conn, testWrite(t, category.ID))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := recurrences.Delete(context.Background(), conn, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := recurrences.Get(context.Background(), conn, created.ID); !errors.Is(err, recurrences.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestListActiveExcludesPausedCommitments(t *testing.T) {
	conn := newTestDB(t)
	category := newCategory(t, conn)
	active, err := recurrences.Create(context.Background(), conn, testWrite(t, category.ID))
	if err != nil {
		t.Fatalf("Create active: %v", err)
	}
	pausedWrite := testWrite(t, category.ID)
	pausedWrite.Name = "Assinatura pausada"
	pausedWrite.IsActive = false
	if _, err := recurrences.Create(context.Background(), conn, pausedWrite); err != nil {
		t.Fatalf("Create paused: %v", err)
	}

	activeOnly, err := recurrences.ListActive(context.Background(), conn)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(activeOnly) != 1 || activeOnly[0].ID != active.ID {
		t.Errorf("expected only the active commitment, got %+v", activeOnly)
	}

	all, err := recurrences.List(context.Background(), conn)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected both commitments in List, got %d", len(all))
	}
}
