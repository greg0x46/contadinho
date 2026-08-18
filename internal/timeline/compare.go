package timeline

import (
	"context"
	"time"

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
// against a Base built with no scenarios — cheap relative to the
// simulation-with-everything call since it's just one more BuildSeries per
// active scenario (see build.go/m5 spec: acceptable for the handful of
// scenarios a user realistically activates at once).
func ScenarioImpact(ctx context.Context, q Querier, base BuildParams, scenarioID string) (Impact, error) {
	scenario, err := scenarios.GetScenario(ctx, q, scenarioID)
	if err != nil {
		return Impact{}, err
	}

	baseParams := base
	baseParams.ScenarioIDs = nil
	baseSeries, err := BuildSeries(ctx, q, baseParams)
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

// Comparison2 is a simple current-vs-previous comparison — named to avoid
// colliding with Comparison (Base vs. Simulation), which compares a
// different pair of things.
type Comparison2 struct {
	Current      decimal.Decimal
	Previous     decimal.Decimal
	DeltaPercent decimal.Decimal // 0 when Previous is exactly zero — a percent change against zero is undefined, not "infinite"
}

func deltaPercent(current, previous decimal.Decimal) decimal.Decimal {
	if previous.IsZero() {
		return decimal.Zero
	}
	return current.Sub(previous).Div(previous.Abs()).Mul(decimal.NewFromInt(100))
}

// MonthOverMonth compares month's Result against the immediately preceding
// calendar month's, both read from an already-computed MonthlyBreakdown.
// ok is false — never a misleading zeroed comparison — when the previous
// month has no row in breakdown at all (MonthlyBreakdown only emits a row
// for months with at least one entry, so "no row" means "no data", not
// "zero movement"), per seção 25: don't show a comparison without enough
// basis for it.
func MonthOverMonth(breakdown []MonthSummary, month time.Time) (*Comparison2, bool) {
	current := monthKey(month)
	previous := current.AddDate(0, -1, 0)

	var currentResult, previousResult decimal.Decimal
	var hasCurrent, hasPrevious bool
	for _, m := range breakdown {
		if m.Month.Equal(current) {
			currentResult = m.Result
			hasCurrent = true
		}
		if m.Month.Equal(previous) {
			previousResult = m.Result
			hasPrevious = true
		}
	}
	if !hasCurrent || !hasPrevious {
		return nil, false
	}
	return &Comparison2{
		Current:      currentResult,
		Previous:     previousResult,
		DeltaPercent: deltaPercent(currentResult, previousResult),
	}, true
}

// YearOverYear compares series' accumulated Result from January through
// throughMonth against priorYearSeries' accumulated Result over the same
// Jan..throughMonth window one year earlier — priorYearSeries must already
// be a Series built for that prior year (BuildSeries with From/To shifted
// back one year), computed by the caller only when the comparison is
// actually requested, not on every report request. ok is false when the
// prior year has no MonthlyBreakdown rows in that window at all.
func YearOverYear(series, priorYearSeries Series, throughMonth time.Time) (*Comparison2, bool) {
	throughKey := monthKey(throughMonth)

	sumThroughMonth := func(breakdown []MonthSummary, through time.Time) (decimal.Decimal, bool) {
		total := decimal.Zero
		found := false
		for _, m := range breakdown {
			if m.Month.After(through) {
				continue
			}
			if m.Month.Year() != through.Year() {
				continue
			}
			total = total.Add(m.Result)
			found = true
		}
		return total, found
	}

	current, hasCurrent := sumThroughMonth(MonthlyBreakdown(series), throughKey)
	priorThrough := time.Date(throughKey.Year()-1, throughKey.Month(), 1, 0, 0, 0, 0, time.UTC)
	previous, hasPrevious := sumThroughMonth(MonthlyBreakdown(priorYearSeries), priorThrough)
	if !hasCurrent || !hasPrevious {
		return nil, false
	}
	return &Comparison2{
		Current:      current,
		Previous:     previous,
		DeltaPercent: deltaPercent(current, previous),
	}, true
}
