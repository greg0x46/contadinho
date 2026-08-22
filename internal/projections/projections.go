// Package projections is the read boundary between Scenario data and
// consumers such as the financial Timeline. It deliberately returns one
// event shape for plans, standalone scenarios, and recurring scenarios; the
// Timeline does not need to know which legacy table supplied an event.
package projections

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/categories"
	"contadinho-go/internal/dates"
	"contadinho-go/internal/money"
	"contadinho-go/internal/recurrences"
	"contadinho-go/internal/scenarios"
	"contadinho-go/internal/timeline/types"
	"contadinho-go/internal/transactions"
)

// SelectionMode makes the difference between the normal Timeline and an
// explicit simulation unambiguous. In particular, an empty ID slice is not
// overloaded to mean either "all active" or "none".
type SelectionMode string

const (
	SelectionActive   SelectionMode = "active"
	SelectionExplicit SelectionMode = "explicit"
)

type ProjectionQuery struct {
	From      time.Time
	To        time.Time
	Selection SelectionMode
	IDs       []string
}

// PlannedTransaction is the canonical event emitted by a Scenario. EventKey
// is stable: it is derived from the scenario identity and the persisted
// transaction/occurrence identity, never generated during a read.
type PlannedTransaction struct {
	ScenarioID        string
	EventKey          string
	ScenarioKind      scenarios.Kind
	Date              time.Time
	Description       string
	Amount            decimal.Decimal
	CategoryID        *string
	CategoryName      string
	Tier              types.Tier
	Source            types.SourceKind
	PayableID         *string
	Realized          bool
	RealizationOrigin string
	Detached          bool
}

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var ErrInvalidSelection = errors.New("invalid projection selection")

// List reads all scenarios selected by query and delegates each kind to its
// adapter. The result is sorted deterministically so callers can merge it
// with real transactions without depending on table or adapter order.
func List(ctx context.Context, q Querier, query ProjectionQuery) ([]PlannedTransaction, error) {
	from, to := dates.Day(query.From), dates.Day(query.To)
	if to.Before(from) {
		return nil, fmt.Errorf("projection range: %w", ErrInvalidSelection)
	}
	selected, err := selectScenarios(ctx, q, query.Selection, query.IDs)
	if err != nil {
		return nil, err
	}

	var out []PlannedTransaction
	plans, err := listPlanProjections(ctx, q, selected, from, to)
	if err != nil {
		return nil, err
	}
	out = append(out, plans...)
	standalone, err := listStandaloneProjections(ctx, q, selected, from, to)
	if err != nil {
		return nil, err
	}
	out = append(out, standalone...)
	recurring, err := listRecurringProjections(ctx, q, selected, from, to)
	if err != nil {
		return nil, err
	}
	out = append(out, recurring...)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date.Equal(out[j].Date) {
			return out[i].EventKey < out[j].EventKey
		}
		return out[i].Date.Before(out[j].Date)
	})
	return out, nil
}

func selectScenarios(ctx context.Context, q Querier, selection SelectionMode, ids []string) ([]scenarios.Scenario, error) {
	switch selection {
	case SelectionActive:
		active := true
		return scenarios.ListScenarios(ctx, q, scenarios.ListFilter{IsActive: &active})
	case SelectionExplicit:
		if len(ids) == 0 {
			return []scenarios.Scenario{}, nil
		}
		return scenarios.ListScenariosByIDs(ctx, q, unique(ids))
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidSelection, selection)
	}
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func listPlanProjections(ctx context.Context, q Querier, selected []scenarios.Scenario, from, to time.Time) ([]PlannedTransaction, error) {
	var out []PlannedTransaction
	for _, scenario := range selected {
		if scenario.Kind != scenarios.KindDebtPlan && scenario.Kind != scenarios.KindReceivablePlan {
			continue
		}
		if scenario.PayableID == nil {
			continue
		}
		transactions, err := scenarios.ListScenarioTransactions(ctx, q, scenario.ID)
		if err != nil {
			return nil, err
		}
		for _, planned := range transactions {
			day := dates.Day(planned.ProjectedAt)
			if day.Before(from) || day.After(to) {
				continue
			}
			realized, origin, err := allocationRealization(ctx, q, planned.ID)
			if err != nil {
				return nil, err
			}
			categoryName := noCategoryName
			if planned.Category != nil && *planned.Category != "" {
				categoryName = *planned.Category
			}
			out = append(out, PlannedTransaction{
				ScenarioID:        scenario.ID,
				EventKey:          transactionEventKey(scenario.ID, planned.ID),
				ScenarioKind:      scenario.Kind,
				Date:              day,
				Description:       planned.Description,
				Amount:            scenarios.SignedAmount(scenario.Kind, planned.Amount),
				CategoryName:      categoryName,
				Tier:              types.TierConfirmado,
				Source:            types.SourcePayablePlan,
				PayableID:         scenario.PayableID,
				Realized:          realized,
				RealizationOrigin: origin,
			})
		}
	}
	return out, nil
}

func listStandaloneProjections(ctx context.Context, q Querier, selected []scenarios.Scenario, from, to time.Time) ([]PlannedTransaction, error) {
	var out []PlannedTransaction
	for _, scenario := range selected {
		if scenario.Kind != scenarios.KindStandalone {
			continue
		}
		transactions, err := scenarios.ListScenarioTransactions(ctx, q, scenario.ID)
		if err != nil {
			return nil, err
		}
		for _, planned := range transactions {
			day := dates.Day(planned.ProjectedAt)
			if day.Before(from) || day.After(to) {
				continue
			}
			realized, origin, err := allocationRealization(ctx, q, planned.ID)
			if err != nil {
				return nil, err
			}
			categoryName := noCategoryName
			if planned.Category != nil && *planned.Category != "" {
				categoryName = *planned.Category
			}
			out = append(out, PlannedTransaction{
				ScenarioID:        scenario.ID,
				EventKey:          transactionEventKey(scenario.ID, planned.ID),
				ScenarioKind:      scenario.Kind,
				Date:              day,
				Description:       planned.Description,
				Amount:            planned.Amount,
				CategoryName:      categoryName,
				Tier:              types.TierHipotetico,
				Source:            types.SourceScenario,
				Realized:          realized,
				RealizationOrigin: origin,
			})
		}
	}
	return out, nil
}

const noCategoryName = "Sem categoria"

func transactionEventKey(scenarioID, transactionID string) string {
	return "scenario:" + scenarioID + ":transaction:" + transactionID
}

func occurrenceEventKey(scenarioID string, day time.Time) string {
	return "scenario:" + scenarioID + ":occurrence:" + dates.Day(day).Format("2006-01-02")
}

// allocationRealization preserves the current ListPlanInstallments rule:
// any positive allocation suppresses the complete planned event. Generic
// rows are authoritative after migration; the old table is a compatibility
// fallback for callers that still create an allocation through the legacy
// endpoint.
func allocationRealization(ctx context.Context, q Querier, scenarioTransactionID string) (bool, string, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT state, origin, allocated_amount
		FROM scenario_realizations
		WHERE scenario_transaction_id = ? AND relation_type = 'allocation'
		ORDER BY created_at DESC`, scenarioTransactionID)
	if err != nil {
		return false, "", err
	}
	found := false
	origin := ""
	realized := false
	for rows.Next() {
		found = true
		var state, rowOrigin, amountRaw string
		if err := rows.Scan(&state, &rowOrigin, &amountRaw); err != nil {
			rows.Close()
			return false, "", err
		}
		amount, parseErr := decimal.NewFromString(amountRaw)
		if parseErr != nil {
			rows.Close()
			return false, "", parseErr
		}
		if state == "linked" && amount.GreaterThan(decimal.Zero) {
			realized = true
			if origin == "" {
				origin = rowOrigin
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, "", err
	}
	rows.Close()
	if found {
		return realized, origin, nil
	}

	// Legacy table rows may exist when the compatibility API is used against
	// a database that predates the generic write path.
	var amountRaw string
	err = q.QueryRowContext(ctx, `
		SELECT allocated_amount FROM scenario_transaction_realizations
		WHERE scenario_transaction_id = ? ORDER BY created_at DESC LIMIT 1`, scenarioTransactionID).Scan(&amountRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	amount, err := decimal.NewFromString(amountRaw)
	return amount.GreaterThan(decimal.Zero), "manual", err
}

type scheduleRow struct {
	schedule     recurrences.RecurringSchedule
	categoryName string
}

func loadSchedule(ctx context.Context, q Querier, scenario scenarios.Scenario) (scheduleRow, error) {
	var (
		cashflowKind, amountRaw, cadence, startRaw string
		categoryID, accountID                      sql.NullString
		dayOfMonth                                 int
		monthOfYear                                sql.NullInt64
		endRaw, categoryName                       sql.NullString
	)
	err := q.QueryRowContext(ctx, `
		SELECT s.cashflow_kind, s.amount, s.category_id, s.account_id, s.cadence,
		       s.day_of_month, s.month_of_year, s.start_date, s.end_date,
		       COALESCE(c.name, '')
		FROM scenario_recurring_schedules s
		LEFT JOIN categories c ON c.id = s.category_id
		WHERE s.scenario_id = ?`, scenario.ID).Scan(
		&cashflowKind, &amountRaw, &categoryID, &accountID, &cadence,
		&dayOfMonth, &monthOfYear, &startRaw, &endRaw, &categoryName,
	)
	if err == nil {
		amount, parseErr := decimal.NewFromString(amountRaw)
		if parseErr != nil {
			return scheduleRow{}, parseErr
		}
		start, parseErr := time.Parse("2006-01-02", startRaw)
		if parseErr != nil {
			return scheduleRow{}, parseErr
		}
		var end *time.Time
		if endRaw.Valid {
			parsed, parseErr := time.Parse("2006-01-02", endRaw.String)
			if parseErr != nil {
				return scheduleRow{}, parseErr
			}
			end = &parsed
		}
		var month *int
		if monthOfYear.Valid {
			value := int(monthOfYear.Int64)
			month = &value
		}
		var category *string
		if categoryID.Valid {
			category = &categoryID.String
		}
		var account *string
		if accountID.Valid {
			account = &accountID.String
		}
		return scheduleRow{schedule: recurrences.RecurringSchedule{
			CashflowKind: recurrences.Kind(cashflowKind), Amount: amount,
			CategoryID: category, AccountID: account, Cadence: recurrences.Cadence(cadence),
			DayOfMonth: dayOfMonth, MonthOfYear: month, StartDate: start, EndDate: end,
		}, categoryName: categoryName.String}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return scheduleRow{}, err
	}
	// Transitional fallback: a legacy row can still be projected even if its
	// schedule was not backfilled (for example in a hand-created test DB).
	var commitmentID string
	if err := q.QueryRowContext(ctx,
		`SELECT recurring_commitment_id FROM recurring_commitment_scenario_map WHERE scenario_id = ?`, scenario.ID,
	).Scan(&commitmentID); err != nil {
		return scheduleRow{}, err
	}
	commitment, err := recurrences.Get(ctx, q, commitmentID)
	if err != nil {
		return scheduleRow{}, err
	}
	cat, err := categories.Get(ctx, q, commitment.CategoryID)
	if err != nil {
		return scheduleRow{}, err
	}
	return scheduleRow{schedule: commitment.Schedule(), categoryName: cat.Name}, nil
}

type occurrenceRealization struct {
	state, origin string
	transactionID *string
}

func occurrenceRealizations(ctx context.Context, q Querier, scenarioID string, from, to time.Time) (map[time.Time]occurrenceRealization, error) {
	result := map[time.Time]occurrenceRealization{}
	rows, err := q.QueryContext(ctx, `
		SELECT occurrence_date, state, origin, transaction_id
		FROM scenario_realizations
		WHERE scenario_id = ? AND relation_type = 'reconciliation'
		  AND occurrence_date >= ? AND occurrence_date <= ?`,
		scenarioID, from.Format("2006-01-02"), to.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dateRaw, state, origin string
		var transactionID sql.NullString
		if err := rows.Scan(&dateRaw, &state, &origin, &transactionID); err != nil {
			return nil, err
		}
		day, err := time.Parse("2006-01-02", dateRaw)
		if err != nil {
			return nil, err
		}
		one := occurrenceRealization{state: state, origin: origin}
		if transactionID.Valid {
			one.transactionID = &transactionID.String
		}
		result[dates.Day(day)] = one
	}
	return result, rows.Err()
}

func listRecurringProjections(ctx context.Context, q Querier, selected []scenarios.Scenario, from, to time.Time) ([]PlannedTransaction, error) {
	var out []PlannedTransaction
	for _, scenario := range selected {
		if scenario.Kind != scenarios.KindRecurring {
			continue
		}
		loaded, err := loadSchedule(ctx, q, scenario)
		if err != nil {
			return nil, err
		}
		occurrences := recurrences.OccurrencesInRange(loaded.schedule, from, to)
		manual, err := occurrenceRealizations(ctx, q, scenario.ID,
			from.AddDate(0, 0, -recurrences.ManualLinkReachDays),
			to.AddDate(0, 0, recurrences.ManualLinkReachDays))
		if err != nil {
			return nil, err
		}
		legacy, err := legacyCommitmentForScenario(ctx, q, scenario.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if errors.Is(err, sql.ErrNoRows) {
			// A canonical recurring Scenario may outlive the legacy
			// recurring_commitments row. Reconstruct the small DTO needed by
			// the existing pure matcher directly from its schedule; this keeps
			// automation keyed by scenario_id from depending on the old table.
			legacy = commitmentFromScenario(scenario, loaded.schedule)
		}
		resolvedByRule, err := resolveLegacyAutomaticOccurrences(ctx, q, scenario.ID, legacy, from, to)
		if err != nil {
			return nil, err
		}
		categoryName := loaded.categoryName
		if categoryName == "" {
			categoryName = noCategoryName
		}
		for _, occurrence := range occurrences {
			day := dates.Day(occurrence.Date)
			resolution := occurrenceRealization{}
			if override, ok := manual[day]; ok {
				resolution = override
			} else if automatic, ok := resolvedByRule[day]; ok {
				resolution = automatic
			}
			amount := occurrence.ExpectedAmount
			if loaded.schedule.CashflowKind == recurrences.KindExpense {
				amount = amount.Neg()
			}
			categoryID := loaded.schedule.CategoryID
			out = append(out, PlannedTransaction{
				ScenarioID:        scenario.ID,
				EventKey:          occurrenceEventKey(scenario.ID, day),
				ScenarioKind:      scenario.Kind,
				Date:              day,
				Description:       scenario.Name,
				Amount:            amount,
				CategoryID:        categoryID,
				CategoryName:      categoryName,
				Tier:              types.TierProjetado,
				Source:            types.SourceRecurring,
				Realized:          resolution.state == "linked" && resolution.transactionID != nil,
				RealizationOrigin: resolution.origin,
				Detached:          resolution.state == "detached",
			})
		}
	}
	return out, nil
}

func commitmentFromScenario(scenario scenarios.Scenario, schedule recurrences.RecurringSchedule) recurrences.RecurringCommitment {
	categoryID := ""
	if schedule.CategoryID != nil {
		categoryID = *schedule.CategoryID
	}
	return recurrences.RecurringCommitment{
		ID: scenario.ID, Name: scenario.Name, Kind: schedule.CashflowKind, Amount: schedule.Amount,
		CategoryID: categoryID, AccountID: schedule.AccountID, Cadence: schedule.Cadence,
		DayOfMonth: schedule.DayOfMonth, MonthOfYear: schedule.MonthOfYear,
		StartDate: schedule.StartDate, EndDate: schedule.EndDate, IsActive: true,
	}
}

func legacyCommitmentForScenario(ctx context.Context, q Querier, scenarioID string) (recurrences.RecurringCommitment, error) {
	var commitmentID string
	if err := q.QueryRowContext(ctx,
		`SELECT recurring_commitment_id FROM recurring_commitment_scenario_map WHERE scenario_id = ?`, scenarioID,
	).Scan(&commitmentID); err != nil {
		return recurrences.RecurringCommitment{}, err
	}
	return recurrences.Get(ctx, q, commitmentID)
}

func resolveLegacyAutomaticOccurrences(ctx context.Context, q Querier, scenarioID string, commitment recurrences.RecurringCommitment, from, to time.Time) (map[time.Time]occurrenceRealization, error) {
	result := map[time.Time]occurrenceRealization{}
	targets, err := automation.ListActiveScenarioReconcileTargets(ctx, q)
	if err != nil {
		return nil, err
	}
	target, ok := targets[scenarioID]
	if !ok {
		return result, nil
	}
	items, err := eligibleCandidates(ctx, q, from.AddDate(0, 0, -recurrences.ManualLinkReachDays), to.AddDate(0, 0, recurrences.ManualLinkReachDays))
	if err != nil {
		return nil, err
	}
	overrides := map[string][]recurrences.Override{}
	if commitment.ID != "" {
		overrides, err = recurrences.ListOverrides(ctx, q, []string{commitment.ID},
			from.AddDate(0, 0, -recurrences.ManualLinkReachDays), to.AddDate(0, 0, recurrences.ManualLinkReachDays))
		if err != nil {
			return nil, err
		}
	}
	legacyCopy := commitment
	legacyCopy.IsActive = true
	reconciler := recurrences.NewReconciler(legacyCopy, target.Conditions, target.LogicOperator, true, overrides[commitment.ID], items)
	for _, resolved := range reconciler.ResolveRange(from, to) {
		if resolved.Reconciled() {
			result[dates.Day(resolved.Occurrence.Date)] = occurrenceRealization{
				state: "linked", origin: string(resolved.Origin), transactionID: resolved.TransactionID,
			}
		}
	}
	return result, nil
}

func eligibleCandidates(ctx context.Context, q Querier, from, to time.Time) ([]transactions.Item, error) {
	fromDate := money.Date{Year: from.Year(), Month: from.Month(), Day: from.Day()}
	toDate := money.Date{Year: to.Year(), Month: to.Month(), Day: to.Day()}
	result, err := transactions.Query(ctx, q, transactions.QueryRequest{
		Timezone: "UTC", GroupBy: money.GroupNone, Page: 1, PageSize: 1_000_000,
		Filters: transactions.Filters{DateFrom: &fromDate, DateTo: &toDate},
	})
	if err != nil {
		return nil, err
	}
	items := make([]transactions.Item, 0, len(result.Items))
	for _, item := range result.Items {
		if item.TotalsEligibility.Included && item.EffectiveMoney != nil && item.OccurredAt != nil {
			items = append(items, item)
		}
	}
	return items, nil
}
