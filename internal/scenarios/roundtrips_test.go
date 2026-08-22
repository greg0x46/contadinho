package scenarios_test

import (
	"context"
	"database/sql"
	"testing"

	"contadinho-go/internal/scenarios"
)

// countingQuerier wraps a *sql.DB and counts round trips, so a test can
// assert a read path's cost in queries rather than in wall-clock time.
type countingQuerier struct {
	db *sql.DB
	n  int
}

func (c *countingQuerier) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	c.n++
	return c.db.QueryContext(ctx, query, args...)
}

func (c *countingQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	c.n++
	return c.db.QueryRowContext(ctx, query, args...)
}

func (c *countingQuerier) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	c.n++
	return c.db.ExecContext(ctx, query, args...)
}

// seedPlan creates a debt plan with n installments and returns its id.
func seedPlan(t *testing.T, conn *sql.DB, n int) string {
	t.Helper()
	ctx := context.Background()
	d := newDebt(t, conn)
	s, err := scenarios.CreateScenario(ctx, conn, scenarios.KindDebtPlan, "Plano", &d.ID)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	for i := 0; i < n; i++ {
		if _, err := scenarios.CreateScenarioTransaction(ctx, conn, s.ID, "Parcela",
			dec(t, "100.00"), date(t, "2026-01-01").AddDate(0, i, 0), nil); err != nil {
			t.Fatalf("CreateScenarioTransaction: %v", err)
		}
	}
	return s.ID
}

// TestSummarizeRoundTripsDoNotGrowWithInstallments guards the batching a
// long plan depends on: allocations are loaded for the whole scenario at
// once, so a 48-parcel plan costs the same round trips as a 2-parcel one.
func TestSummarizeRoundTripsDoNotGrowWithInstallments(t *testing.T) {
	conn := newTestDB(t)
	scenarioID := seedPlan(t, conn, 48)

	c := &countingQuerier{db: conn}
	if _, err := scenarios.Summarize(context.Background(), c, scenarioID, date(t, "2026-06-01")); err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	// The installments, then every allocation across them.
	if c.n > 2 {
		t.Errorf("Summarize over 48 installments = %d queries, want <= 2 — "+
			"a per-installment realization query has crept back in", c.n)
	}
}

// TestListPlanInstallmentsRoundTripsDoNotGrowWithPlansOrInstallments is the
// same guard for the timeline's projection path, which runs on every
// timeline request — once for the base series and once more per simulated
// scenario. It seeds several plans on purpose: a single-plan fixture would
// pass just as happily against a query-per-plan loop.
func TestListPlanInstallmentsRoundTripsDoNotGrowWithPlansOrInstallments(t *testing.T) {
	conn := newTestDB(t)
	for i := 0; i < 6; i++ {
		seedPlan(t, conn, 48)
	}

	c := &countingQuerier{db: conn}
	got, err := scenarios.ListPlanInstallments(context.Background(), c,
		date(t, "2026-01-01"), date(t, "2030-01-01"))
	if err != nil {
		t.Fatalf("ListPlanInstallments: %v", err)
	}
	if len(got) != 6*48 {
		t.Fatalf("len(installments) = %d, want %d — the batching must not drop a plan", len(got), 6*48)
	}
	// The plans, then every installment across them, then their allocations.
	if c.n > 3 {
		t.Errorf("ListPlanInstallments over 6 plans x 48 installments = %d queries, want <= 3 — "+
			"a per-plan or per-installment query has crept back in", c.n)
	}
}
