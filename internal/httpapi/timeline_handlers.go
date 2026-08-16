package httpapi

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"contadinho-go/internal/money"
	"contadinho-go/internal/timeline"
)

func timelineUnavailableProblem(w http.ResponseWriter) {
	writeProblem(w, 503, "timeline-unavailable", "Relatório financeiro temporariamente indisponível", "Tente novamente em instantes.")
}

func invalidTimelineProblem(w http.ResponseWriter, detail string) {
	writeProblem(w, 422, "invalid-timeline-request", "Parâmetros do relatório inválidos", detail)
}

type timelineEntryDTO struct {
	Date         string  `json:"date"`
	Description  string  `json:"description"`
	Amount       string  `json:"amount"`
	CategoryID   *string `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Tier         string  `json:"tier"`
	Source       string  `json:"source"`
	SourceRefID  string  `json:"source_ref_id"`
	ScenarioID   *string `json:"scenario_id"`
}

func timelineEntryToDTO(e timeline.Entry) timelineEntryDTO {
	return timelineEntryDTO{
		Date:         e.Date.Format(dateOnlyLayout),
		Description:  e.Description,
		Amount:       money.CanonicalDecimal(e.Amount),
		CategoryID:   e.CategoryID,
		CategoryName: e.CategoryName,
		Tier:         string(e.Tier),
		Source:       string(e.Source),
		SourceRefID:  e.SourceRefID,
		ScenarioID:   e.ScenarioID,
	}
}

type timelineDayPointDTO struct {
	Date       string `json:"date"`
	Balance    string `json:"balance"`
	Inflow     string `json:"inflow"`
	Outflow    string `json:"outflow"`
	LowestTier string `json:"lowest_tier"`
}

func timelineDayPointToDTO(p timeline.DayPoint) timelineDayPointDTO {
	return timelineDayPointDTO{
		Date:       p.Date.Format(dateOnlyLayout),
		Balance:    money.CanonicalDecimal(p.Balance),
		Inflow:     money.CanonicalDecimal(p.Inflow),
		Outflow:    money.CanonicalDecimal(p.Outflow),
		LowestTier: string(p.LowestTier),
	}
}

type timelineSeriesDTO struct {
	Points          []timelineDayPointDTO `json:"points"`
	Entries         []timelineEntryDTO    `json:"entries"`
	StartingBalance string                `json:"starting_balance"`
	LowestBalance   timelineDayPointDTO   `json:"lowest_balance"`
	FirstNegative   *string               `json:"first_negative"`
}

func timelineSeriesToDTO(s timeline.Series) timelineSeriesDTO {
	points := make([]timelineDayPointDTO, len(s.Points))
	for i, p := range s.Points {
		points[i] = timelineDayPointToDTO(p)
	}
	entries := make([]timelineEntryDTO, len(s.Entries))
	for i, e := range s.Entries {
		entries[i] = timelineEntryToDTO(e)
	}
	var firstNegative *string
	if s.FirstNegative != nil {
		formatted := s.FirstNegative.Format(dateOnlyLayout)
		firstNegative = &formatted
	}
	return timelineSeriesDTO{
		Points:          points,
		Entries:         entries,
		StartingBalance: money.CanonicalDecimal(s.StartingBalance),
		LowestBalance:   timelineDayPointToDTO(s.LowestBalance),
		FirstNegative:   firstNegative,
	}
}

type monthSummaryDTO struct {
	Month   string `json:"month"`
	Income  string `json:"income"`
	Expense string `json:"expense"`
	Result  string `json:"result"`
}

func monthSummaryToDTO(m timeline.MonthSummary) monthSummaryDTO {
	return monthSummaryDTO{
		Month:   m.Month.Format(dateOnlyLayout),
		Income:  money.CanonicalDecimal(m.Income),
		Expense: money.CanonicalDecimal(m.Expense),
		Result:  money.CanonicalDecimal(m.Result),
	}
}

type categoryImpactDTO struct {
	CategoryID   *string `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Amount       string  `json:"amount"`
	Percentage   string  `json:"percentage"`
}

func categoryImpactToDTO(c timeline.CategoryImpact) categoryImpactDTO {
	return categoryImpactDTO{
		CategoryID:   c.CategoryID,
		CategoryName: c.CategoryName,
		Amount:       money.CanonicalDecimal(c.Amount),
		Percentage:   money.CanonicalDecimal(c.Percentage),
	}
}

type scenarioImpactDTO struct {
	ScenarioID   string `json:"scenario_id"`
	ScenarioName string `json:"scenario_name"`
	Delta        string `json:"delta"`
}

func scenarioImpactToDTO(i timeline.Impact) scenarioImpactDTO {
	return scenarioImpactDTO{ScenarioID: i.ScenarioID, ScenarioName: i.ScenarioName, Delta: money.CanonicalDecimal(i.Delta)}
}

type monthAmountDTO struct {
	Month  string `json:"month"`
	Amount string `json:"amount"`
}

func monthAmountToDTO(m timeline.MonthAmount) monthAmountDTO {
	return monthAmountDTO{Month: m.Month.Format(dateOnlyLayout), Amount: money.CanonicalDecimal(m.Amount)}
}

type comparison2DTO struct {
	Current      string `json:"current"`
	Previous     string `json:"previous"`
	DeltaPercent string `json:"delta_percent"`
}

func comparison2ToDTO(c *timeline.Comparison2) *comparison2DTO {
	if c == nil {
		return nil
	}
	return &comparison2DTO{
		Current:      money.CanonicalDecimal(c.Current),
		Previous:     money.CanonicalDecimal(c.Previous),
		DeltaPercent: money.CanonicalDecimal(c.DeltaPercent),
	}
}

// comparison2FromResult adapts a (comparison, ok) pair — ok=false means
// "not enough data", which must render as no comparison at all, never a
// zeroed one.
func comparison2FromResult(comparison *timeline.Comparison2, ok bool) *comparison2DTO {
	if !ok {
		return nil
	}
	return comparison2ToDTO(comparison)
}

// timelineResponseDTO is the full payload: the canonical Series plus the
// aggregates every presentation layer reads instead of recomputing locally
// (see the umbrella spec's reconciliation principle). Simulation/
// ScenarioImpacts populate only when scenario_ids was non-empty;
// YearOverYear/CategoryEvolution populate only when explicitly requested
// (each costs an extra BuildSeries call); MonthOverMonth is always
// computed (free — it only reads the MonthlyBreakdown already built).
// Any of the three comparison/evolution fields is null when there isn't
// enough data for it — never a misleading zeroed value (seção 25).
type timelineResponseDTO struct {
	Base              timelineSeriesDTO   `json:"base"`
	MonthlyBreakdown  []monthSummaryDTO   `json:"monthly_breakdown"`
	CategoryBreakdown []categoryImpactDTO `json:"category_breakdown"`
	Simulation        *timelineSeriesDTO  `json:"simulation"`
	ScenarioImpacts   []scenarioImpactDTO `json:"scenario_impacts"`
	MonthOverMonth    *comparison2DTO     `json:"month_over_month"`
	YearOverYear      *comparison2DTO     `json:"year_over_year"`
	CategoryEvolution []monthAmountDTO    `json:"category_evolution"`
}

func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// handleGetTimeline serves GET /api/timeline?reference_date=...&from=...&to=...
// &account_ids=...&category_ids=...&card_numbers=...&scenario_ids=...
// scenario_ids empty (the default) returns {base, simulation: null,
// scenario_impacts: []}; non-empty adds Simulation (Base + those scenarios)
// and one Impact per scenario, each isolated against Base — never all
// standalone scenarios that exist, only the ones actually selected.
func handleGetTimeline(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		referenceRaw := query.Get("reference_date")
		fromRaw := query.Get("from")
		toRaw := query.Get("to")
		if referenceRaw == "" || fromRaw == "" || toRaw == "" {
			invalidTimelineProblem(w, "Informe reference_date, from e to.")
			return
		}
		reference, err := time.Parse(dateOnlyLayout, referenceRaw)
		if err != nil {
			invalidTimelineProblem(w, "reference_date inválida.")
			return
		}
		from, err := time.Parse(dateOnlyLayout, fromRaw)
		if err != nil {
			invalidTimelineProblem(w, "from inválida.")
			return
		}
		to, err := time.Parse(dateOnlyLayout, toRaw)
		if err != nil {
			invalidTimelineProblem(w, "to inválida.")
			return
		}
		if to.Before(from) {
			invalidTimelineProblem(w, "to não pode ser anterior a from.")
			return
		}

		baseParams := timeline.BuildParams{
			From: from, To: to, ReferenceDate: reference,
			AccountIDs:  splitCSV(query.Get("account_ids")),
			CategoryIDs: splitCSV(query.Get("category_ids")),
			CardNumbers: splitCSV(query.Get("card_numbers")),
		}
		scenarioIDs := splitCSV(query.Get("scenario_ids"))

		series, err := timeline.BuildSeries(r.Context(), conn, baseParams)
		if err != nil {
			timelineUnavailableProblem(w)
			return
		}

		months := timeline.MonthlyBreakdown(series)
		monthDTOs := make([]monthSummaryDTO, len(months))
		for i, m := range months {
			monthDTOs[i] = monthSummaryToDTO(m)
		}
		categories := timeline.CategoryBreakdown(series, reference)
		categoryDTOs := make([]categoryImpactDTO, len(categories))
		for i, c := range categories {
			categoryDTOs[i] = categoryImpactToDTO(c)
		}

		response := timelineResponseDTO{
			Base:              timelineSeriesToDTO(series),
			MonthlyBreakdown:  monthDTOs,
			CategoryBreakdown: categoryDTOs,
			ScenarioImpacts:   []scenarioImpactDTO{},
			MonthOverMonth:    comparison2FromResult(timeline.MonthOverMonth(months, reference)),
		}

		if len(scenarioIDs) > 0 {
			simulationParams := baseParams
			simulationParams.ScenarioIDs = scenarioIDs
			simulationSeries, err := timeline.BuildSeries(r.Context(), conn, simulationParams)
			if err != nil {
				timelineUnavailableProblem(w)
				return
			}
			simulationDTO := timelineSeriesToDTO(simulationSeries)
			response.Simulation = &simulationDTO

			for _, scenarioID := range scenarioIDs {
				impact, err := timeline.ScenarioImpact(r.Context(), conn, baseParams, scenarioID)
				if err != nil {
					timelineUnavailableProblem(w)
					return
				}
				response.ScenarioImpacts = append(response.ScenarioImpacts, scenarioImpactToDTO(impact))
			}
		}

		// year_over_year=true costs a second BuildSeries call (the prior
		// year's series), so it only runs when explicitly asked for.
		if query.Get("year_over_year") == "true" {
			priorYearParams := baseParams
			priorYearParams.From = from.AddDate(-1, 0, 0)
			priorYearParams.To = to.AddDate(-1, 0, 0)
			priorYearSeries, err := timeline.BuildSeries(r.Context(), conn, priorYearParams)
			if err != nil {
				timelineUnavailableProblem(w)
				return
			}
			response.YearOverYear = comparison2FromResult(timeline.YearOverYear(series, priorYearSeries, reference))
		}

		// category_evolution_id, when present, is a category UUID or the
		// literal "none" for "Sem categoria" — free (reuses series, no
		// extra query), so no separate opt-in flag is needed.
		if evolutionID := query.Get("category_evolution_id"); evolutionID != "" {
			var categoryID *string
			if evolutionID != "none" {
				categoryID = &evolutionID
			}
			months := timeline.CategoryEvolution(series, categoryID)
			response.CategoryEvolution = make([]monthAmountDTO, len(months))
			for i, m := range months {
				response.CategoryEvolution[i] = monthAmountToDTO(m)
			}
		}

		writeJSON(w, http.StatusOK, response)
	}
}
