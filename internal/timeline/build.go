package timeline

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/categories"
	"contadinho-go/internal/money"
	"contadinho-go/internal/payables"
	"contadinho-go/internal/recurrences"
	"contadinho-go/internal/scenarios"
	"contadinho-go/internal/transactions"
)

// Querier is satisfied by both *sql.DB and *sql.Tx — identical in shape to
// transactions.Querier, reused as an alias so BuildSeries can pass q
// straight through to transactions.Query without a type assertion.
type Querier = transactions.Querier

// BuildParams scopes BuildSeries — dates are calendar days (time-of-day
// ignored), AccountIDs/CategoryIDs/CardNumbers empty means "no filter".
// ScenarioIDs is stricter: empty means "no hypothetical entries at all",
// never "all scenarios" — an inactive (unselected) standalone scenario must
// never leak into a Series (see m5-simulacao-com-cenarios.md's seção 30).
type BuildParams struct {
	From, To      time.Time
	ReferenceDate time.Time
	AccountIDs    []string
	CategoryIDs   []string
	CardNumbers   []string
	ScenarioIDs   []string
}

func dayOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// startingBalance sums financial_accounts.balance, optionally scoped to
// accountIDs — the timeline's t=today anchor, always the provider's
// authoritative balance, never recomputed locally. Credit card accounts are
// excluded: their balance is owed debt, not cash on hand.
func startingBalance(ctx context.Context, q Querier, accountIDs []string) (decimal.Decimal, error) {
	query := `SELECT balance FROM financial_accounts WHERE balance IS NOT NULL AND (account_type IS NULL OR account_type != 'CREDIT')`
	args := []any{}
	if len(accountIDs) > 0 {
		placeholders := make([]string, len(accountIDs))
		for i, id := range accountIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		query += ` AND id IN (` + joinPlaceholders(placeholders) + `)`
	}
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return decimal.Decimal{}, err
	}
	defer rows.Close()
	total := decimal.Zero
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return decimal.Decimal{}, err
		}
		amount, err := decimal.NewFromString(raw)
		if err != nil {
			return decimal.Decimal{}, fmt.Errorf("parse account balance %q: %w", raw, err)
		}
		total = total.Add(amount)
	}
	return total, rows.Err()
}

func joinPlaceholders(placeholders []string) string {
	out := placeholders[0]
	for _, p := range placeholders[1:] {
		out += "," + p
	}
	return out
}

// eligibleRealItems loads real transactions in [from, to], applying the
// accountIDs/categoryIDs/cardNumbers filters that transactions.Filters
// can't express directly (it only takes one AccountID/CategoryID, not a
// slice). Returned alongside realEntries (its Entry projection) so
// recurrenceEntries can reconcile against the very same set without a
// second query, per the M4 spec.
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
		if !item.TotalsEligibility.Included || item.EffectiveMoney == nil || item.OccurredAt == nil {
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
// payables.CardDueDates.ProjectedEntryDate.
func realEntries(ctx context.Context, q Querier, items []transactions.Item) ([]Entry, error) {
	creditAccounts, err := payables.CreditAccountIDs(ctx, q)
	if err != nil {
		return nil, err
	}
	transactionIDs := make([]string, 0, len(items))
	for _, item := range items {
		if creditAccounts[item.Account.ID] {
			transactionIDs = append(transactionIDs, item.ID)
		}
	}
	cardMetadata, err := payables.CardMetadataByTransaction(ctx, q, transactionIDs)
	if err != nil {
		return nil, err
	}
	var dueDates payables.CardDueDates
	if len(transactionIDs) > 0 {
		dueDates, err = payables.FetchCardDueDates(ctx, q)
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
		categoryName := "Sem categoria"
		if item.InternalCategory != nil {
			categoryID = &item.InternalCategory.ID
			categoryName = item.InternalCategory.Name
		}
		description := ""
		if item.Description != nil {
			description = *item.Description
		}
		date := dayOnly(*item.OccurredAt)
		tier := TierRealizado
		if creditAccounts[item.Account.ID] {
			date = dayOnly(dueDates.ProjectedEntryDate(item.Account.ID, *item.OccurredAt, cardMetadata[item.ID]))
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
		})
	}
	return entries, nil
}

// payablePlanEntries scans every Scenario{Kind: debt_plan|receivable_plan}
// and its unrealized ScenarioTransactions within [from, to] — Tier
// Confirmado, since it's a real debt/receivable with a planned date, not a
// hypothesis. Amount's sign follows the backing Payable's Kind (debt =
// outflow, receivable = inflow).
func payablePlanEntries(ctx context.Context, q Querier, from, to time.Time) ([]Entry, error) {
	plans, err := scenarios.ListPayablePlanScenarios(ctx, q)
	if err != nil {
		return nil, err
	}

	payableKind := map[string]payables.Kind{}
	var entries []Entry
	for _, plan := range plans {
		if plan.PayableID == nil {
			continue
		}
		kind, cached := payableKind[*plan.PayableID]
		if !cached {
			p, err := payables.Get(ctx, q, *plan.PayableID)
			if err != nil {
				return nil, err
			}
			kind = p.Kind
			payableKind[*plan.PayableID] = kind
		}

		installments, err := scenarios.ListScenarioTransactions(ctx, q, plan.ID)
		if err != nil {
			return nil, err
		}
		for _, installment := range installments {
			day := dayOnly(installment.ProjectedAt)
			if day.Before(from) || day.After(to) {
				continue
			}
			realizations, err := scenarios.ListRealizationsForTransaction(ctx, q, installment.ID)
			if err != nil {
				return nil, err
			}
			realizedTotal := decimal.Zero
			for _, r := range realizations {
				realizedTotal = realizedTotal.Add(r.AllocatedAmount)
			}
			if !realizedTotal.IsZero() {
				continue // already (at least partly) realized — the real transaction already carries it, via realEntries
			}

			amount := installment.Amount
			if kind == payables.KindDebt {
				amount = amount.Neg()
			}
			categoryName := "Sem categoria"
			if installment.Category != nil && *installment.Category != "" {
				categoryName = *installment.Category
			}
			entries = append(entries, Entry{
				Date:         day,
				Description:  installment.Description,
				Amount:       amount,
				CategoryName: categoryName,
				Tier:         TierConfirmado,
				Source:       SourcePayablePlan,
				SourceRefID:  installment.ID,
			})
		}
	}
	return entries, nil
}

// recurrenceEntries produces one Entry per RecurringCommitment occurrence
// in [from, to] for every active commitment: Confirmado either way, using
// the real transaction's own amount if ResolveOccurrence found one already
// posted this cycle (still "confirmed", just with the actual value), or the
// commitment's expected amount if not — never a second query, reconciled
// against the same realCandidates realEntries already loaded. A commitment
// with no linked automation rule (see internal/automation.
// ListActiveReconcileTargets) has nothing to resolve against, so its
// occurrences always keep the expected amount — this is what lets a
// recurring commitment exist independent of any automation.
func recurrenceEntries(ctx context.Context, q Querier, from, to time.Time, realCandidates []transactions.Item) ([]Entry, error) {
	commitments, err := recurrences.ListActive(ctx, q)
	if err != nil {
		return nil, err
	}

	reconcileTargets, err := automation.ListActiveReconcileTargets(ctx, q)
	if err != nil {
		return nil, err
	}

	categoryName := map[string]string{}
	var entries []Entry
	for _, commitment := range commitments {
		name, cached := categoryName[commitment.CategoryID]
		if !cached {
			cat, err := categories.Get(ctx, q, commitment.CategoryID)
			if err != nil {
				return nil, err
			}
			name = cat.Name
			categoryName[commitment.CategoryID] = name
		}
		candidates := make([]transactions.Item, 0, len(realCandidates))
		for _, item := range realCandidates {
			if item.InternalCategory == nil || item.InternalCategory.ID != commitment.CategoryID {
				continue
			}
			if commitment.AccountID != nil && item.Account.ID != *commitment.AccountID {
				continue
			}
			candidates = append(candidates, item)
		}

		for _, occurrence := range recurrences.OccurrencesInRange(commitment, from, to) {
			amount := occurrence.ExpectedAmount
			sourceRefID := commitment.ID + ":" + occurrence.Date.Format("2006-01-02")
			if rule, hasRule := reconcileTargets[commitment.ID]; hasRule {
				if matched, ok := recurrences.ResolveOccurrence(rule.Conditions, rule.LogicOperator, candidates); ok {
					matchedAmount, err := decimal.NewFromString(matched.EffectiveMoney.Value)
					if err != nil {
						return nil, fmt.Errorf("parse recurrence match amount %q: %w", matched.EffectiveMoney.Value, err)
					}
					amount = matchedAmount.Abs()
				}
			}
			if commitment.Kind == recurrences.KindExpense {
				amount = amount.Neg()
			}
			categoryID := commitment.CategoryID
			entries = append(entries, Entry{
				Date:         dayOnly(occurrence.Date),
				Description:  commitment.Name,
				Amount:       amount,
				CategoryID:   &categoryID,
				CategoryName: name,
				Tier:         TierConfirmado,
				Source:       SourceRecurring,
				SourceRefID:  sourceRefID,
			})
		}
	}
	return entries, nil
}

// scenarioEntries loads ScenarioTransactions for exactly the
// Scenario{Kind: standalone} named in scenarioIDs within [from, to] — never
// "every standalone scenario that exists". An empty scenarioIDs returns no
// entries without touching the database, which is what makes an unselected
// scenario invisible by construction rather than by a filter someone could
// forget to apply. Tier: Hipotético.
func scenarioEntries(ctx context.Context, q Querier, scenarioIDs []string, from, to time.Time) ([]Entry, error) {
	if len(scenarioIDs) == 0 {
		return nil, nil
	}
	selected, err := scenarios.ListScenariosByIDs(ctx, q, scenarioIDs)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, scenario := range selected {
		if scenario.Kind != scenarios.KindStandalone {
			continue // defense in depth: only standalone scenarios are valid simulation input
		}
		transactions, err := scenarios.ListScenarioTransactions(ctx, q, scenario.ID)
		if err != nil {
			return nil, err
		}
		for _, st := range transactions {
			day := dayOnly(st.ProjectedAt)
			if day.Before(from) || day.After(to) {
				continue
			}
			categoryName := "Sem categoria"
			if st.Category != nil && *st.Category != "" {
				categoryName = *st.Category
			}
			scenarioID := scenario.ID
			entries = append(entries, Entry{
				Date:         day,
				Description:  st.Description,
				Amount:       st.Amount,
				CategoryName: categoryName,
				Tier:         TierHipotetico,
				Source:       SourceScenario,
				SourceRefID:  st.ID,
				ScenarioID:   &scenarioID,
			})
		}
	}
	return entries, nil
}

// BuildSeries merges every source this phase supports into a single Series
// — see the package doc for why nothing downstream recomputes this.
func BuildSeries(ctx context.Context, q Querier, params BuildParams) (Series, error) {
	from, to, reference := dayOnly(params.From), dayOnly(params.To), dayOnly(params.ReferenceDate)

	balance, err := startingBalance(ctx, q, params.AccountIDs)
	if err != nil {
		return Series{}, err
	}

	items, err := eligibleRealItems(ctx, q, from, to, params.AccountIDs, params.CategoryIDs, params.CardNumbers)
	if err != nil {
		return Series{}, err
	}
	entries, err := realEntries(ctx, q, items)
	if err != nil {
		return Series{}, err
	}
	planEntries, err := payablePlanEntries(ctx, q, from, to)
	if err != nil {
		return Series{}, err
	}
	entries = append(entries, planEntries...)
	commitmentEntries, err := recurrenceEntries(ctx, q, from, to, items)
	if err != nil {
		return Series{}, err
	}
	entries = append(entries, commitmentEntries...)
	hypotheticalEntries, err := scenarioEntries(ctx, q, params.ScenarioIDs, from, to)
	if err != nil {
		return Series{}, err
	}
	entries = append(entries, hypotheticalEntries...)

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
