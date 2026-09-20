package timeline

import (
	"context"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/scenarios"
)

// Comparison is Base vs. Simulation (Base + active scenarios), compared
// point-by-point at the two figures the umbrella spec calls out (seção
// 18): the final balance and the lowest balance along the path.
type Comparison struct {
	BaseFinalBalance        decimal.Decimal
	SimulationFinalBalance  decimal.Decimal
	Impact                  decimal.Decimal // Simulation - Base
	BaseLowestBalance       DayPoint
	SimulationLowestBalance DayPoint
}

func finalBalance(series Series) decimal.Decimal {
	if len(series.Points) == 0 {
		return series.StartingBalance
	}
	return series.Points[len(series.Points)-1].Balance
}

// CompareBaseVsSimulation compares two Series already built with the same
// filters — one with ScenarioIDs empty (Base), one with the active set
// (Simulation) — never recomputing either itself.
func CompareBaseVsSimulation(base, simulation Series) Comparison {
	baseFinal := finalBalance(base)
	simulationFinal := finalBalance(simulation)
	return Comparison{
		BaseFinalBalance:        baseFinal,
		SimulationFinalBalance:  simulationFinal,
		Impact:                  simulationFinal.Sub(baseFinal),
		BaseLowestBalance:       base.LowestBalance,
		SimulationLowestBalance: simulation.LowestBalance,
	}
}

// Impact isolates one scenario's effect on the final balance of the period
// analyzed (seção 19: "apenas o impacto dentro do período atualmente
// analisado").
type Impact struct {
	ScenarioID   string
	ScenarioName string
	Delta        decimal.Decimal
}

// ScenarioImpact runs BuildSeries with exactly [scenarioID] active (in
// addition to base's own filters) and returns the delta in final balance
// against baseSeries — one BuildSeries per scenario, so a handler comparing
// several of them at once pays for each scenario and not for rebuilding the
// Base alongside every one.
//
// baseSeries must be what BuildSeries returns for base with ScenarioIDs
// cleared — same filters, same window, no scenarios. That is a contract
// this function cannot check: a baseSeries built with different filters
// would still subtract cleanly and report a plausible, wrong delta. Every
// caller that wants impacts already renders the Base, so it holds exactly
// that series; base.ScenarioIDs itself is ignored here and replaced with
// the one scenario being measured.
func ScenarioImpact(ctx context.Context, q Querier, base BuildParams, baseSeries Series, scenarioID string) (Impact, error) {
	scenario, err := scenarios.GetScenario(ctx, q, scenarioID)
	if err != nil {
		return Impact{}, err
	}

	withScenario := base
	withScenario.ScenarioIDs = []string{scenarioID}
	withScenarioSeries, err := BuildSeries(ctx, q, withScenario)
	if err != nil {
		return Impact{}, err
	}

	return Impact{
		ScenarioID:   scenario.ID,
		ScenarioName: scenario.Name,
		Delta:        finalBalance(withScenarioSeries).Sub(finalBalance(baseSeries)),
	}, nil
}
