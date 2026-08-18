package timeline_test

import (
	"context"
	"testing"

	"contadinho-go/internal/scenarios"
	"contadinho-go/internal/timeline"
)

func TestBuildSeriesScenarioEntriesOnlySelectedIDs(t *testing.T) {
	f := newFixture(t)
	f.addAccount("1000.00")
	ctx := context.Background()

	trip, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindStandalone, "Viagem", nil)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, f.conn, trip.ID, "Passagem", decT(t, "-800.00"), date(t, "2026-09-10"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}
	job, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindStandalone, "Novo emprego", nil)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, f.conn, job.ID, "Salário extra", decT(t, "500.00"), date(t, "2026-09-05"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}

	// Not selecting any scenario must produce zero hypothetical entries —
	// an unselected scenario is invisible by construction (seção 30).
	base, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-08-15"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15"),
	})
	if err != nil {
		t.Fatalf("BuildSeries base: %v", err)
	}
	for _, e := range base.Entries {
		if e.Source == timeline.SourceScenario {
			t.Errorf("base series must not include scenario entries, got %+v", e)
		}
	}

	onlyTrip, err := timeline.BuildSeries(ctx, f.conn, timeline.BuildParams{
		From: date(t, "2026-08-15"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15"),
		ScenarioIDs: []string{trip.ID},
	})
	if err != nil {
		t.Fatalf("BuildSeries onlyTrip: %v", err)
	}
	var scenarioEntries int
	for _, e := range onlyTrip.Entries {
		if e.Source == timeline.SourceScenario {
			scenarioEntries++
			if e.ScenarioID == nil || *e.ScenarioID != trip.ID {
				t.Errorf("entry ScenarioID = %v, want %s", e.ScenarioID, trip.ID)
			}
			if e.Tier != timeline.TierHipotetico {
				t.Errorf("entry Tier = %s, want hipotetico", e.Tier)
			}
		}
	}
	if scenarioEntries != 1 {
		t.Fatalf("scenario entries = %d, want 1 (only Viagem, Novo emprego not selected)", scenarioEntries)
	}
}

func TestCompareBaseVsSimulation(t *testing.T) {
	f := newFixture(t)
	f.addAccount("1000.00")
	ctx := context.Background()

	trip, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindStandalone, "Viagem", nil)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, f.conn, trip.ID, "Passagem", decT(t, "-300.00"), date(t, "2026-09-10"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}

	params := timeline.BuildParams{From: date(t, "2026-08-15"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15")}
	base, err := timeline.BuildSeries(ctx, f.conn, params)
	if err != nil {
		t.Fatalf("BuildSeries base: %v", err)
	}
	withTrip := params
	withTrip.ScenarioIDs = []string{trip.ID}
	simulation, err := timeline.BuildSeries(ctx, f.conn, withTrip)
	if err != nil {
		t.Fatalf("BuildSeries simulation: %v", err)
	}

	comparison := timeline.CompareBaseVsSimulation(base, simulation)
	if comparison.Impact.String() != "-300" {
		t.Errorf("Impact = %s, want -300", comparison.Impact.String())
	}
	if comparison.SimulationFinalBalance.Sub(comparison.BaseFinalBalance).String() != "-300" {
		t.Errorf("final balance delta = %s, want -300", comparison.SimulationFinalBalance.Sub(comparison.BaseFinalBalance).String())
	}
}

func TestScenarioImpactIndividualAndCombined(t *testing.T) {
	f := newFixture(t)
	f.addAccount("1000.00")
	ctx := context.Background()

	trip, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindStandalone, "Viagem", nil)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, f.conn, trip.ID, "Passagem", decT(t, "-300.00"), date(t, "2026-09-10"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}
	job, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindStandalone, "Novo emprego", nil)
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if _, err := scenarios.CreateScenarioTransaction(ctx, f.conn, job.ID, "Salário extra", decT(t, "500.00"), date(t, "2026-09-05"), nil); err != nil {
		t.Fatalf("CreateScenarioTransaction: %v", err)
	}

	params := timeline.BuildParams{
		From: date(t, "2026-08-15"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-08-15"),
		ScenarioIDs: []string{trip.ID, job.ID},
	}

	tripImpact, err := timeline.ScenarioImpact(ctx, f.conn, params, trip.ID)
	if err != nil {
		t.Fatalf("ScenarioImpact trip: %v", err)
	}
	if tripImpact.Delta.String() != "-300" || tripImpact.ScenarioName != "Viagem" {
		t.Errorf("tripImpact = %+v, want -300 Viagem", tripImpact)
	}

	jobImpact, err := timeline.ScenarioImpact(ctx, f.conn, params, job.ID)
	if err != nil {
		t.Fatalf("ScenarioImpact job: %v", err)
	}
	if jobImpact.Delta.String() != "500" || jobImpact.ScenarioName != "Novo emprego" {
		t.Errorf("jobImpact = %+v, want 500 Novo emprego", jobImpact)
	}

	// Combined simulation impact must equal the sum of the two individual
	// deltas (the two scenarios don't interact).
	combined, err := timeline.BuildSeries(ctx, f.conn, params)
	if err != nil {
		t.Fatalf("BuildSeries combined: %v", err)
	}
	baseParams := params
	baseParams.ScenarioIDs = nil
	base, err := timeline.BuildSeries(ctx, f.conn, baseParams)
	if err != nil {
		t.Fatalf("BuildSeries base: %v", err)
	}
	comparison := timeline.CompareBaseVsSimulation(base, combined)
	wantCombined := tripImpact.Delta.Add(jobImpact.Delta)
	if !comparison.Impact.Equal(wantCombined) {
		t.Errorf("combined Impact = %s, want %s (sum of individual deltas)", comparison.Impact, wantCombined)
	}

	// Removing a scenario from the active set recalculates correctly:
	// with only "Novo emprego" active, the impact is just its own delta.
	onlyJob := params
	onlyJob.ScenarioIDs = []string{job.ID}
	onlyJobSeries, err := timeline.BuildSeries(ctx, f.conn, onlyJob)
	if err != nil {
		t.Fatalf("BuildSeries onlyJob: %v", err)
	}
	afterRemoval := timeline.CompareBaseVsSimulation(base, onlyJobSeries)
	if !afterRemoval.Impact.Equal(jobImpact.Delta) {
		t.Errorf("after removing Viagem, Impact = %s, want %s", afterRemoval.Impact, jobImpact.Delta)
	}
}
