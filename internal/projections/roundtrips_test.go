package projections_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/projections"
	"contadinho-go/internal/recurrences"
	"contadinho-go/internal/rules"
	"contadinho-go/internal/scenarios"
)

// countingQuerier counts every round trip List makes, so the test can assert
// on the shape of the read rather than on wall-clock time.
type countingQuerier struct {
	inner *sql.DB
	n     atomic.Int64
}

func (c *countingQuerier) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	c.n.Add(1)
	return c.inner.ExecContext(ctx, query, args...)
}

func (c *countingQuerier) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	c.n.Add(1)
	return c.inner.QueryContext(ctx, query, args...)
}

func (c *countingQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	c.n.Add(1)
	return c.inner.QueryRowContext(ctx, query, args...)
}

// seedProjectionLoad creates count recurring scenarios and count standalone
// scenarios, the latter with six installments each — the shape a Timeline of a
// real user looks like.
func seedProjectionLoad(t *testing.T, conn *sql.DB, count int) []string {
	t.Helper()
	ctx := context.Background()
	var recurringIDs []string
	for i := 0; i < count; i++ {
		commitment, err := recurrences.Create(ctx, conn, recurrences.Write{
			Name: fmt.Sprintf("Recorrente %d", i), Kind: recurrences.KindExpense,
			Amount: projectionDecimal(t, "100.00"), CategoryID: projectionCategoryID,
			Cadence: recurrences.CadenceMonthly, DayOfMonth: 15,
			StartDate: projectionDate(t, "2026-01-01"), IsActive: true,
		})
		if err != nil {
			t.Fatalf("recurrences.Create: %v", err)
		}
		recurringIDs = append(recurringIDs, commitment.ID)

		scenario, err := scenarios.CreateScenario(ctx, conn, scenarios.KindStandalone, fmt.Sprintf("Cenário %d", i), nil)
		if err != nil {
			t.Fatalf("CreateScenario: %v", err)
		}
		for j := 0; j < 6; j++ {
			if _, err := scenarios.CreateScenarioTransaction(ctx, conn, scenario.ID,
				fmt.Sprintf("Parcela %d", j), projectionDecimal(t, "-50.00"),
				projectionDate(t, "2026-02-10"), nil); err != nil {
				t.Fatalf("CreateScenarioTransaction: %v", err)
			}
		}
	}
	return recurringIDs
}

func countListRoundTrips(t *testing.T, conn *sql.DB) int64 {
	t.Helper()
	counter := &countingQuerier{inner: conn}
	if _, err := projections.List(context.Background(), counter, projections.ProjectionQuery{
		From: projectionDate(t, "2026-01-01"), To: projectionDate(t, "2026-06-30"),
		Selection: projections.SelectionActive,
	}); err != nil {
		t.Fatalf("List: %v", err)
	}
	return counter.n.Load()
}

// TestListRoundTripsDoNotGrowWithSelection pins the property that makes List
// usable as the dashboard's read: every per-scenario lookup it needs — the
// schedules, the installments, the allocations, the occurrence decisions, the
// reconcile rules, the candidate transactions — is batched into one query for
// the whole selection.
//
// The number itself is not the contract; staying flat is. A regression to one
// query per scenario (or, worse, one full transaction scan per scenario) shows
// up here as a count that grows with the seed size.
func TestListRoundTripsDoNotGrowWithSelection(t *testing.T) {
	small := projectionDB(t)
	seedProjectionLoad(t, small, 1)
	baseline := countListRoundTrips(t, small)

	large := projectionDB(t)
	seedProjectionLoad(t, large, 16)
	scaled := countListRoundTrips(t, large)

	if scaled != baseline {
		t.Errorf("round trips grew with the selection: 1 scenario pair = %d, 16 pairs = %d", baseline, scaled)
	}
}

// TestListRoundTripsDoNotGrowWithReconcileRules covers the expensive half
// separately: once any scenario is targeted by a reconcile rule, List has to
// load the period's eligible transactions to run the matcher. That load is a
// property of the period, not of the scenario, so it must happen once no
// matter how many targeted scenarios there are.
func TestListRoundTripsDoNotGrowWithReconcileRules(t *testing.T) {
	ctx := context.Background()

	withOne := projectionDB(t)
	oneID := seedProjectionLoad(t, withOne, 1)
	reconcileRuleFor(ctx, t, withOne, oneID)
	baseline := countListRoundTrips(t, withOne)

	withMany := projectionDB(t)
	manyIDs := seedProjectionLoad(t, withMany, 8)
	reconcileRuleFor(ctx, t, withMany, manyIDs)
	scaled := countListRoundTrips(t, withMany)

	if scaled != baseline {
		t.Errorf("round trips grew with the number of reconcile targets: 1 target = %d, 8 targets = %d", baseline, scaled)
	}
}

func reconcileRuleFor(ctx context.Context, t *testing.T, conn *sql.DB, scenarioIDs []string) {
	t.Helper()
	for i, scenarioID := range scenarioIDs {
		id := scenarioID
		if _, err := automation.Create(ctx, conn, automation.Write{
			Name:          fmt.Sprintf("Concilia %d", i),
			IsActive:      true,
			LogicOperator: rules.LogicAnd,
			Conditions: []rules.Condition{
				{Field: rules.FieldDescription, Operator: rules.OperatorContains, Value: "aluguel"},
			},
			Actions: []automation.ActionWrite{{Type: automation.ActionReconcile, ScenarioID: &id}},
		}); err != nil {
			t.Fatalf("automation.Create: %v", err)
		}
	}
}
