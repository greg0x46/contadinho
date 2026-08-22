package timeline

import (
	"context"
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
// never leak into a Series (see .specs/motores-de-dominio.md, princípio 3).
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
		})
	}
	return entries, nil
}

// payablePlanEntries projects the unrealized installments of every
// payable-backed plan in [from, to] — Tier Confirmado, since it's a real
// debt/receivable with a planned date, not a hypothesis.
//
// The plan's cash-flow direction and its link to a Payable both stay inside
// package scenarios (Scenario.PayableID/Kind): this package deliberately
// does not know payables at all, per .specs/motores-de-dominio.md section 6.
func payablePlanEntries(ctx context.Context, q Querier, from, to time.Time) ([]Entry, error) {
	installments, err := scenarios.ListPlanInstallments(ctx, q, from, to)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(installments))
	for _, installment := range installments {
		categoryName := noCategoryName
		if installment.Category != nil && *installment.Category != "" {
			categoryName = *installment.Category
		}
		entries = append(entries, Entry{
			Date:         installment.ProjectedAt,
			Description:  installment.Description,
			Amount:       installment.Amount,
			CategoryName: categoryName,
			Tier:         TierConfirmado,
			Source:       SourcePayablePlan,
			SourceRefID:  installment.TransactionID,
		})
	}
	return entries, nil
}

// recurrenceEntries produces one Entry per *unreconciled* RecurringCommitment
// occurrence in [from, to] for every active commitment, at Tier Projetado:
// the occurrence is scheduled and carries its expected amount, but nothing
// real has been observed for it yet.
//
// A reconciled occurrence emits no Entry at all — the real transaction is
// already in the series via realEntries, and emitting the occurrence too
// would count the same money twice. This mirrors payablePlanEntries'
// treatment of a realized installment.
//
// Whether an occurrence counts as reconciled is decided entirely by
// recurrences.Reconciler, which is also what the HTTP layer reads: a manual
// override (the user linking a transaction by hand, or detaching the
// occurrence) wins over the automation rule, and the rule matches within the
// occurrence's own month otherwise. Keeping that decision in one place is
// what makes "desconciliar" in the UI actually move this series — the whole
// point of the override existing.
//
// A commitment with no linked automation rule (see
// internal/automation.ListActiveReconcileTargets) and no manual override has
// nothing to resolve against, so its occurrences are always emitted — this is
// what lets a recurring commitment exist independent of any automation.
//
// Boundary note: a manual link may point at a transaction in a neighbouring
// month, in which case the occurrence is suppressed here while the real
// transaction lands in the adjacent month's total. The occurrence is
// suppressed regardless, because the reconciliation is a fact about the
// occurrence; the candidate window the HTTP layer offers is deliberately
// narrow so the effect stays bounded to one month boundary.
func recurrenceEntries(ctx context.Context, q Querier, from, to time.Time, realCandidates []transactions.Item) ([]Entry, error) {
	commitments, err := recurrences.ListActive(ctx, q)
	if err != nil {
		return nil, err
	}

	reconcileTargets, err := automation.ListActiveReconcileTargets(ctx, q)
	if err != nil {
		return nil, err
	}

	commitmentIDs := make([]string, len(commitments))
	for i, commitment := range commitments {
		commitmentIDs[i] = commitment.ID
	}
	// Widened past [from, to] on purpose: the Reconciler uses the overrides to
	// keep the rule off transactions another occurrence already claims by
	// hand, and a hand-linked transaction can sit in a neighbouring month.
	// Loading only this window's overrides would let a transaction linked to
	// (say) February's occurrence be re-matched by the rule to January's,
	// suppressing the wrong projection. The margin matches the candidate
	// window the HTTP layer offers, which is what bounds how far a manual
	// link can reach.
	overrides, err := recurrences.ListOverrides(ctx, q, commitmentIDs,
		from.AddDate(0, 0, -recurrences.ManualLinkReachDays),
		to.AddDate(0, 0, recurrences.ManualLinkReachDays))
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
		rule, hasRule := reconcileTargets[commitment.ID]
		reconciler := recurrences.NewReconciler(
			commitment, rule.Conditions, rule.LogicOperator, hasRule,
			overrides[commitment.ID], realCandidates,
		)

		for _, resolved := range reconciler.ResolveRange(from, to) {
			if resolved.Reconciled() {
				continue
			}
			amount := resolved.Occurrence.ExpectedAmount
			if commitment.Kind == recurrences.KindExpense {
				amount = amount.Neg()
			}
			categoryID := commitment.CategoryID
			entries = append(entries, Entry{
				Date:         dates.Day(resolved.Occurrence.Date),
				Description:  commitment.Name,
				Amount:       amount,
				CategoryID:   &categoryID,
				CategoryName: name,
				Tier:         TierProjetado,
				Source:       SourceRecurring,
				SourceRefID:  commitment.ID + ":" + resolved.Occurrence.Date.Format("2006-01-02"),
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
			day := dates.Day(st.ProjectedAt)
			if day.Before(from) || day.After(to) {
				continue
			}
			categoryName := noCategoryName
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
	from, to, reference := dates.Day(params.From), dates.Day(params.To), dates.Day(params.ReferenceDate)

	// The t=today anchor: cash on hand, from the one implementation net
	// worth's asset side also reads (see transactions.CashOnHand).
	balance, err := transactions.CashOnHand(ctx, q, params.AccountIDs)
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
