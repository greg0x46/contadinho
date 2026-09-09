package timeline

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestHistoricalLowestDoesNotCreateAFutureWarning(t *testing.T) {
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	points := []DayPoint{
		{Date: day, Balance: decimal.NewFromInt(100), LowestTier: TierRealizado},
		{Date: day.AddDate(0, 0, 1), Balance: decimal.NewFromInt(-50), LowestTier: TierRealizado},
		{Date: day.AddDate(0, 0, 2), Balance: decimal.NewFromInt(200), LowestTier: TierRealizado},
	}
	lowest, negative := lowestAndFirstNegative(points, day.AddDate(0, 1, 0))
	if !lowest.Date.Equal(points[1].Date) || !lowest.Balance.Equal(points[1].Balance) || lowest.LowestTier != TierRealizado {
		t.Fatalf("lowest = %+v, want historical minimum %+v", lowest, points[1])
	}
	if negative != nil {
		t.Fatalf("historical negative balance became a future warning: %v", negative)
	}

	// A window including the reference date still excludes historical lows.
	lowest, negative = lowestAndFirstNegative(points, points[2].Date)
	if !lowest.Date.Equal(points[2].Date) || negative != nil {
		t.Fatalf("current minimum = %+v, negative = %v", lowest, negative)
	}
}
