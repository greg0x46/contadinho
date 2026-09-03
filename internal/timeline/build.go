package timeline

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/dates"
	"contadinho-go/internal/money"
	"contadinho-go/internal/projections"
	"contadinho-go/internal/transactions"
)

// Querier is satisfied by both *sql.DB and *sql.Tx — identical in shape to
// transactions.Querier, reused as an alias so BuildSeries can pass q
// straight through to transactions.Query without a type assertion.
type Querier = transactions.Querier

// BuildParams scopes BuildSeries — dates are calendar days (time-of-day
// ignored), AccountIDs/CategoryIDs/CardNumbers empty means "no filter".
// ScenarioIDs selects an explicit simulation set when non-empty. When empty,
// the unified projector selects active scenarios, so inactive standalone
// scenarios remain out of the default Timeline without special-case logic.
type BuildParams struct {
	From, To      time.Time
	ReferenceDate time.Time
	AccountIDs    []string
	CategoryIDs   []string
	CardNumbers   []string
	ScenarioIDs   []string
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// eligibleRealItems loads real transactions in [from, to], applying the
// accountIDs/categoryIDs/cardNumbers filters that transactions.Filters
// can't express directly (it only takes one AccountID/CategoryID, not a
// slice). Projected events are resolved by internal/projections; this helper
// intentionally knows only about the financial transaction side.
//
// The filter is MovesCash, not Included: this series is a balance over time,
// not an income/expense report. A transfer between the user's own accounts is
// deliberately outside the totals, but the money really did leave the origin
// account — and buildPoints anchors the curve by subtracting past entries
// from today's real balance, so dropping one leaves every day before it short
// by the transfer's full amount. On an account-filtered series, where the
// counterpart leg is not in the set at all, that error never cancels out.
func eligibleRealItems(ctx context.Context, q Querier, from, to time.Time, accountIDs, categoryIDs, cardNumbers []string) ([]transactions.Item, error) {
	fromDate := money.Date{Year: from.Year(), Month: from.Month(), Day: from.Day()}
	toDate := money.Date{Year: to.Year(), Month: to.Month(), Day: to.Day()}

	result, err := transactions.Query(ctx, q, transactions.QueryRequest{
		Timezone: "UTC",
		GroupBy:  money.GroupNone,
		Page:     1,
		PageSize: 1_000_000,
		Filters: transactions.Filters{
			DateFrom: &fromDate,
			DateTo:   &toDate,
		},
	})
	if err != nil {
		return nil, err
	}

	items := make([]transactions.Item, 0, len(result.Items))
	for _, item := range result.Items {
		if !item.TotalsEligibility.MovesCash() || item.EffectiveMoney == nil || item.OccurredAt == nil {
			continue
		}
		if len(accountIDs) > 0 && !contains(accountIDs, item.Account.ID) {
			continue
		}
		if len(categoryIDs) > 0 {
			if item.InternalCategory == nil || !contains(categoryIDs, item.InternalCategory.ID) {
				continue
			}
		}
		if len(cardNumbers) > 0 {
			if item.Card == nil || !contains(cardNumbers, item.Card.Number) {
				continue
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// realEntries projects eligible real transactions into TierRealizado/
// SourceReal entries. Credit card transactions are the exception: the
// purchase already happened, but the cash only leaves the paying account
// when the bill is due, so their entry is dated (and ranked) as
// TierConfirmado at the projected due date instead of occurred_at — see
// transactions.CardDueDates.ProjectedEntryDate.
func realEntries(ctx context.Context, q Querier, items []transactions.Item) ([]Entry, error) {
	creditAccounts, err := transactions.CreditAccountIDs(ctx, q)
	if err != nil {
		return nil, err
	}
	transactionIDs := make([]string, 0, len(items))
	for _, item := range items {
		if creditAccounts[item.Account.ID] {
			transactionIDs = append(transactionIDs, item.ID)
		}
	}
	cardMetadata, err := transactions.CardMetadataByTransaction(ctx, q, transactionIDs)
	if err != nil {
		return nil, err
	}
	var dueDates transactions.CardDueDates
	if len(transactionIDs) > 0 {
		dueDates, err = transactions.FetchCardDueDates(ctx, q)
		if err != nil {
			return nil, err
		}
	}

	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		amount, err := decimal.NewFromString(item.EffectiveMoney.Value)
		if err != nil {
			return nil, fmt.Errorf("parse transaction amount %q: %w", item.EffectiveMoney.Value, err)
		}
		// Classification, not the raw value's sign, decides direction —
		// mirrors currencyTotalsFor's Value.Abs() in transactions/query.go.
		amount = amount.Abs()
		if item.Classification == money.Outflow {
			amount = amount.Neg()
		}
		var categoryID *string
		categoryName := noCategoryName
		if item.InternalCategory != nil {
			categoryID = &item.InternalCategory.ID
			categoryName = item.InternalCategory.Name
		}
		description := ""
		if item.Description != nil {
			description = *item.Description
		}
		date := dates.Day(*item.OccurredAt)
		tier := TierRealizado
		if creditAccounts[item.Account.ID] {
			date = dates.Day(dueDates.ProjectedEntryDate(item.Account.ID, *item.OccurredAt, cardMetadata[item.ID]))
			tier = TierConfirmado
		}
		entries = append(entries, Entry{
			Date:         date,
			Description:  description,
			Amount:       amount,
			CategoryID:   categoryID,
			CategoryName: categoryName,
			Tier:         tier,
			Source:       SourceReal,
			SourceRefID:  item.ID,
			EventKey:     "transaction:" + item.ID,
		})
	}
	return entries, nil
}

// BuildSeries merges real transactions and the unified Scenario projection
// stream into one Series. Scenario kinds, schedules, automation, links, and
// realization precedence all stay behind projections.List.
func BuildSeries(ctx context.Context, q Querier, params BuildParams) (Series, error) {
	from, to, reference := dates.Day(params.From), dates.Day(params.To), dates.Day(params.ReferenceDate)

	// The t=today anchor: cash on hand, from the one implementation net
	// worth's asset side also reads (see transactions.CashOnHand).
	balance, err := transactions.CashOnHand(ctx, q, params.AccountIDs)
	if err != nil {
		return Series{}, err
	}

	// The fetch reaches back before from because a credit-card purchase is
	// re-dated to its bill's due date (see realEntries): a purchase made in
	// the cycle or two before the window is paid inside it, and querying
	// only [from, to] would leave that bill — real money about to leave the
	// account — out of the projection. Entries whose projected date still
	// falls outside the window are dropped by withinWindow below.
	items, err := eligibleRealItems(ctx, q, from.AddDate(0, -2, 0), to, params.AccountIDs, params.CategoryIDs, params.CardNumbers)
	if err != nil {
		return Series{}, err
	}
	entries, err := realEntries(ctx, q, items)
	if err != nil {
		return Series{}, err
	}
	selection := projections.SelectionActive
	if len(params.ScenarioIDs) > 0 {
		selection = projections.SelectionExplicit
	}
	planned, err := projections.List(ctx, q, projections.ProjectionQuery{
		From: from, To: to, Selection: selection, IDs: params.ScenarioIDs,
	})
	if err != nil {
		return Series{}, err
	}
	for _, event := range planned {
		// A linked planned event is represented by the real transaction in
		// entries already. Keeping the suppression here makes it impossible
		// for a future adapter to accidentally reintroduce double counting.
		if event.Realized {
			continue
		}
		scenarioID := event.ScenarioID
		entries = append(entries, Entry{
			Date: event.Date, Description: event.Description, Amount: event.Amount,
			CategoryID: event.CategoryID, CategoryName: event.CategoryName,
			Tier: CertaintyTier(event.Tier), Source: SourceKind(event.Source),
			SourceRefID: event.EventKey, EventKey: event.EventKey, ScenarioID: &scenarioID,
		})
	}

	entries = withinWindow(entries, from, to)

	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Date.Before(entries[j].Date) })

	points := buildPoints(entries, balance, from, to, reference)
	lowest, firstNegative := lowestAndFirstNegative(points, reference)

	return Series{
		Points:          points,
		Entries:         entries,
		StartingBalance: balance,
		LowestBalance:   lowest,
		FirstNegative:   firstNegative,
	}, nil
}

// withinWindow drops entries whose date falls outside [from, to]. Only a
// real credit-card entry can land there: it is re-dated from occurred_at to
// its bill's due date (see realEntries), which can sit before from or past
// to even though the purchase itself was inside the queried window. Such an
// entry belongs to another window's series, and leaving it here would do
// more than list a row the chart cannot show: buildPoints anchors the
// balance by subtracting every entry dated on or before reference, so an
// entry before from would be subtracted from the anchor and never added
// back by any day in the loop — shifting every single point, and with them
// LowestBalance, away from the real balance.
func withinWindow(entries []Entry, from, to time.Time) []Entry {
	kept := entries[:0]
	for _, e := range entries {
		if e.Date.Before(from) || e.Date.After(to) {
			continue
		}
		kept = append(kept, e)
	}
	return kept
}

// buildPoints produces one DayPoint per calendar day in [from, to],
// anchoring the running balance so it equals startingBalance on
// reference (see the package's build.go comment for the reasoning): the
// balance right after every entry up to and including reference must sum
// to startingBalance, and every entry after reference adds onward from
// there — so a purely historical M3 series (all entries <= reference)
// reads as "how did the balance get to what it is today", while future
// entries (from M4 on) read as a forward projection from today.
func buildPoints(entries []Entry, startingBalance decimal.Decimal, from, to, reference time.Time) []DayPoint {
	pastSum := decimal.Zero
	for _, e := range entries {
		if !e.Date.After(reference) {
			pastSum = pastSum.Add(e.Amount)
		}
	}
	anchor := startingBalance.Sub(pastSum)

	byDay := map[time.Time]struct {
		net, inflow, outflow decimal.Decimal
		weakestTier          CertaintyTier
	}{}
	for _, e := range entries {
		d := byDay[e.Date]
		d.net = d.net.Add(e.Amount)
		if e.Amount.IsPositive() {
			d.inflow = d.inflow.Add(e.Amount)
		} else {
			d.outflow = d.outflow.Add(e.Amount)
		}
		if weaker(e.Tier, d.weakestTier) {
			d.weakestTier = e.Tier
		}
		byDay[e.Date] = d
	}

	var points []DayPoint
	running := anchor
	tier := TierRealizado
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		d := byDay[day]
		running = running.Add(d.net)
		if d.weakestTier != "" {
			tier = d.weakestTier // a day with no entries of its own keeps the balance's last-known certainty
		}
		points = append(points, DayPoint{
			Date:       day,
			Balance:    running,
			Inflow:     d.inflow,
			Outflow:    d.outflow,
			LowestTier: tier,
		})
	}
	return points
}

// tierRank orders certainty from most to least sure — used to track, per
// day, the weakest tier among that day's own entries (a day with a
// Projetado entry reads as Projetado even if a Realizado entry also landed
// that day, since the balance is only as certain as its shakiest input).
var tierRank = map[CertaintyTier]int{
	TierRealizado:  0,
	TierConfirmado: 1,
	TierProjetado:  2,
	TierHipotetico: 3,
}

func weaker(candidate, current CertaintyTier) bool {
	return current == "" || tierRank[candidate] > tierRank[current]
}

// lowestAndFirstNegative only looks at points from reference onward — the
// path still ahead — per the spec: a low balance the user already lived
// through isn't a warning, only one still coming is.
func lowestAndFirstNegative(points []DayPoint, reference time.Time) (DayPoint, *time.Time) {
	var lowest DayPoint
	var firstNegative *time.Time
	first := true
	for _, p := range points {
		if p.Date.Before(reference) {
			continue
		}
		if first || p.Balance.LessThan(lowest.Balance) {
			lowest = p
			first = false
		}
		if firstNegative == nil && p.Balance.IsNegative() {
			d := p.Date
			firstNegative = &d
		}
	}
	return lowest, firstNegative
}
