package transactions

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/categories"
	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
)

// row is one joined financial_transactions/financial_accounts/
// transaction_inclusion_decisions/transaction_category_decisions/categories
// record, straight off the wire before any domain logic runs.
type row struct {
	id                      string
	accountID               string
	externalID              string
	description             *string
	amount                  *decimal.Decimal
	amountInAccountCurrency *decimal.Decimal
	currencyCode            *string
	occurredAt              *time.Time
	providerStatus          *string
	movementType            *string
	sourceCategory          *string

	accountName         *string
	accountInstitution  *string
	accountCurrencyCode *string

	inclusionState     *string
	inclusionChangedAt *time.Time
	inclusionOrigin    *string
	inclusionRuleName  *string

	categoryDecisionCategoryID *string
	categoryDecisionChangedAt  *time.Time
	categoryDecisionOrigin     *string

	categoryID       *string
	categoryName     *string
	categoryKind     *string
	categoryIsActive *bool
}

// view is a row with every rule from package money already applied. Query
// fetches every transaction unfiltered (SQLite has none of Postgres' date_trunc
// / AT TIME ZONE machinery to push grouping into SQL, and at the scale of a
// single user's personal transactions doing the equivalent work in Go is both
// simpler and fast enough), then filters, sorts, paginates, and aggregates
// entirely in Go.
type view struct {
	row            row
	classification money.Classification
	effective      *money.EffectiveMoney
	included       bool
	reason         *money.EligibilityReason
	period         money.Period
}

func fetchAllViews(ctx context.Context, q Querier, query QueryRequest) ([]view, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT
			ft.id, ft.account_id, ft.external_id, ft.description, ft.amount,
			ft.amount_in_account_currency, ft.currency_code, ft.occurred_at,
			ft.provider_status, ft.movement_type, ft.source_category,
			fa.name, fa.institution, fa.currency_code,
			tid.state, tid.changed_at, tid.origin, tid.rule_name,
			tcd.category_id, tcd.changed_at, tcd.origin,
			cat.id, cat.name, cat.kind, cat.is_active
		FROM financial_transactions ft
		JOIN financial_accounts fa ON fa.id = ft.account_id
		LEFT JOIN transaction_inclusion_decisions tid ON tid.transaction_id = ft.id
		LEFT JOIN transaction_category_decisions tcd ON tcd.transaction_id = ft.id
		LEFT JOIN categories cat ON cat.id = tcd.category_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	var views []view
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		v, err := buildView(r, query)
		if err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return views, nil
}

func scanRow(rows *sql.Rows) (row, error) {
	var (
		r                          row
		amount, amountAcct         sql.NullString
		currencyCode               sql.NullString
		occurredAt                 sql.NullString
		providerStatus             sql.NullString
		movementType               sql.NullString
		sourceCategory             sql.NullString
		description                sql.NullString
		accountName                sql.NullString
		accountInstitution         sql.NullString
		accountCurrencyCode        sql.NullString
		inclusionState             sql.NullString
		inclusionChangedAt         sql.NullString
		inclusionOrigin            sql.NullString
		inclusionRuleName          sql.NullString
		categoryDecisionCategoryID sql.NullString
		categoryDecisionChangedAt  sql.NullString
		categoryDecisionOrigin     sql.NullString
		categoryID                 sql.NullString
		categoryName               sql.NullString
		categoryKind               sql.NullString
		categoryIsActive           sql.NullBool
	)
	err := rows.Scan(
		&r.id, &r.accountID, &r.externalID, &description, &amount,
		&amountAcct, &currencyCode, &occurredAt,
		&providerStatus, &movementType, &sourceCategory,
		&accountName, &accountInstitution, &accountCurrencyCode,
		&inclusionState, &inclusionChangedAt, &inclusionOrigin, &inclusionRuleName,
		&categoryDecisionCategoryID, &categoryDecisionChangedAt, &categoryDecisionOrigin,
		&categoryID, &categoryName, &categoryKind, &categoryIsActive,
	)
	if err != nil {
		return row{}, err
	}

	r.description = nullString(description)
	r.currencyCode = nullString(currencyCode)
	r.providerStatus = nullString(providerStatus)
	r.movementType = nullString(movementType)
	r.sourceCategory = nullString(sourceCategory)
	r.accountName = nullString(accountName)
	r.accountInstitution = nullString(accountInstitution)
	r.accountCurrencyCode = nullString(accountCurrencyCode)
	r.inclusionState = nullString(inclusionState)
	r.inclusionOrigin = nullString(inclusionOrigin)
	r.inclusionRuleName = nullString(inclusionRuleName)
	r.categoryDecisionCategoryID = nullString(categoryDecisionCategoryID)
	r.categoryDecisionOrigin = nullString(categoryDecisionOrigin)
	r.categoryID = nullString(categoryID)
	r.categoryName = nullString(categoryName)
	r.categoryKind = nullString(categoryKind)
	if categoryIsActive.Valid {
		r.categoryIsActive = &categoryIsActive.Bool
	}

	if r.amount, err = nullDecimal(amount); err != nil {
		return row{}, err
	}
	if r.amountInAccountCurrency, err = nullDecimal(amountAcct); err != nil {
		return row{}, err
	}
	if r.occurredAt, err = db.ParseNullTime(occurredAt); err != nil {
		return row{}, err
	}
	if r.inclusionChangedAt, err = db.ParseNullTime(inclusionChangedAt); err != nil {
		return row{}, err
	}
	if r.categoryDecisionChangedAt, err = db.ParseNullTime(categoryDecisionChangedAt); err != nil {
		return row{}, err
	}
	return r, nil
}

func nullString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func nullDecimal(ns sql.NullString) (*decimal.Decimal, error) {
	if !ns.Valid {
		return nil, nil
	}
	d, err := decimal.NewFromString(ns.String)
	if err != nil {
		return nil, fmt.Errorf("parse decimal %q: %w", ns.String, err)
	}
	return &d, nil
}

func buildView(r row, query QueryRequest) (view, error) {
	classification := money.Classify(r.movementType)
	effective := money.SelectEffectiveMoney(r.amountInAccountCurrency, r.accountCurrencyCode, r.amount, r.currencyCode)

	inclusionState := money.Considered
	if r.inclusionState != nil && *r.inclusionState == string(money.Ignored) {
		inclusionState = money.Ignored
	}

	included, reason := money.Eligibility(classification, r.providerStatus, effective, inclusionState)

	period, err := money.PeriodFor(r.occurredAt, query.GroupBy, query.Timezone)
	if err != nil {
		return view{}, err
	}

	return view{
		row:            r,
		classification: classification,
		effective:      effective,
		included:       included,
		reason:         reason,
		period:         period,
	}, nil
}

// matches mirrors query_sql.filter_clauses, evaluated in Go against an
// already-built view instead of as SQL predicates.
func matches(v view, f Filters, bounds *dateBounds) bool {
	if bounds != nil {
		if v.row.occurredAt == nil {
			return false
		}
		t := *v.row.occurredAt
		if t.Before(bounds.start) || !t.Before(bounds.end) {
			return false
		}
	}
	if f.Description != nil {
		if v.row.description == nil {
			return false
		}
		if !strings.Contains(strings.ToLower(*v.row.description), strings.ToLower(*f.Description)) {
			return false
		}
	}
	if f.AccountID != nil && v.row.accountID != *f.AccountID {
		return false
	}
	if f.Institution != nil {
		if v.row.accountInstitution == nil || *v.row.accountInstitution != *f.Institution {
			return false
		}
	}
	if f.CategoryID != nil {
		if v.row.categoryDecisionCategoryID == nil || *v.row.categoryDecisionCategoryID != *f.CategoryID {
			return false
		}
	}
	if f.Uncategorized && v.row.categoryDecisionCategoryID != nil {
		return false
	}
	if f.ProviderStatus != nil {
		if v.row.providerStatus == nil || *v.row.providerStatus != *f.ProviderStatus {
			return false
		}
	}
	if f.AmountMin != nil || f.AmountMax != nil {
		if v.effective == nil {
			return false
		}
		abs := v.effective.Value.Abs()
		if f.AmountMin != nil && abs.LessThan(*f.AmountMin) {
			return false
		}
		if f.AmountMax != nil && abs.GreaterThan(*f.AmountMax) {
			return false
		}
	}
	if f.Classification != nil && v.classification != *f.Classification {
		return false
	}
	return true
}

type dateBounds struct {
	start time.Time
	end   time.Time
}

func computeDateBounds(f Filters, timezone string) (*dateBounds, error) {
	if f.DateFrom == nil || f.DateTo == nil {
		return nil, nil
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", timezone, err)
	}
	from := f.DateFrom
	to := f.DateTo
	start := time.Date(from.Year, from.Month, from.Day, 0, 0, 0, 0, loc).UTC()
	end := time.Date(to.Year, to.Month, to.Day, 0, 0, 0, 0, loc).AddDate(0, 0, 1).UTC()
	return &dateBounds{start: start, end: end}, nil
}

// Query mirrors query_transactions: it resolves every domain rule, filters,
// sorts by occurred_at desc (SQLite already treats NULL as least-than-any-value,
// so DESC naturally sorts undated transactions last, matching the reference's
// explicit nulls_last), paginates, and computes per-bucket and overall
// currency totals over eligible transactions only.
func Query(ctx context.Context, q Querier, query QueryRequest) (Result, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 50
	}

	allViews, err := fetchAllViews(ctx, q, query)
	if err != nil {
		return Result{}, err
	}
	storedTotal := len(allViews)

	bounds, err := computeDateBounds(query.Filters, query.Timezone)
	if err != nil {
		return Result{}, err
	}

	filtered := make([]view, 0, len(allViews))
	for _, v := range allViews {
		if matches(v, query.Filters, bounds) {
			filtered = append(filtered, v)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i].row, filtered[j].row
		if a.occurredAt == nil && b.occurredAt == nil {
			return a.id > b.id
		}
		if a.occurredAt == nil {
			return false // nil sorts last (equivalent to -infinity in DESC order)
		}
		if b.occurredAt == nil {
			return true
		}
		if !a.occurredAt.Equal(*b.occurredAt) {
			return a.occurredAt.After(*b.occurredAt)
		}
		return a.id > b.id
	})

	totalItems := len(filtered)
	startIndex := (query.Page - 1) * query.PageSize
	pageEnd := startIndex + query.PageSize
	if pageEnd > totalItems {
		pageEnd = totalItems
	}
	var pageViews []view
	if startIndex < totalItems {
		pageViews = filtered[startIndex:pageEnd]
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + query.PageSize - 1) / query.PageSize
	}

	items := make([]Item, len(pageViews))
	for i, v := range pageViews {
		items[i] = toItem(v)
	}

	groups := buildGroups(filtered, pageViews, startIndex, pageEnd)
	overallTotals := buildOverallTotals(filtered)

	availableFilters, err := buildAvailableFilters(ctx, q, allViews)
	if err != nil {
		return Result{}, err
	}

	return Result{
		ConfirmedAt: time.Now().UTC(),
		StoredTotal: storedTotal,
		Page: Page{
			Number:     query.Page,
			Size:       query.PageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
		Items:            items,
		Totals:           overallTotals,
		Groups:           groups,
		AvailableFilters: availableFilters,
	}, nil
}

func toItem(v view) Item {
	r := v.row
	item := Item{
		ID:             r.id,
		ExternalID:     r.externalID,
		OccurredAt:     r.occurredAt,
		Description:    r.description,
		SourceCategory: r.sourceCategory,
		MovementType:   r.movementType,
		ProviderStatus: r.providerStatus,
		Classification: v.classification,
		CurrencyCode:   r.currencyCode,
		Account: AccountSummary{
			ID:           r.accountID,
			Name:         r.accountName,
			Institution:  r.accountInstitution,
			CurrencyCode: r.accountCurrencyCode,
		},
		Inclusion: Inclusion{
			State:     inclusionStateOf(r),
			ChangedAt: r.inclusionChangedAt,
			Origin:    stringOr(r.inclusionOrigin, "manual"),
			RuleName:  r.inclusionRuleName,
		},
		TotalsEligibility: TotalsEligibility{Included: v.included, Reason: v.reason},
		GroupKey:          v.period.Key,
	}
	if r.amount != nil {
		s := money.CanonicalDecimal(*r.amount)
		item.Amount = &s
	}
	if r.amountInAccountCurrency != nil {
		s := money.CanonicalDecimal(*r.amountInAccountCurrency)
		item.AmountInAccountCurrency = &s
	}
	if v.effective != nil {
		item.EffectiveMoney = &EffectiveMoneyView{
			Value:        money.CanonicalDecimal(v.effective.Value),
			CurrencyCode: v.effective.CurrencyCode,
			Source:       v.effective.Source,
		}
	}
	if r.categoryID != nil && r.categoryDecisionCategoryID != nil {
		item.InternalCategory = &InternalCategory{
			ID:        *r.categoryID,
			Name:      stringOr(r.categoryName, ""),
			Kind:      money.CategoryKind(stringOr(r.categoryKind, "")),
			IsActive:  r.categoryIsActive != nil && *r.categoryIsActive,
			Origin:    stringOr(r.categoryDecisionOrigin, "automatic"),
			ChangedAt: timeOr(r.categoryDecisionChangedAt),
		}
	}
	return item
}

func inclusionStateOf(r row) money.InclusionState {
	if r.inclusionState != nil && *r.inclusionState == string(money.Ignored) {
		return money.Ignored
	}
	return money.Considered
}

func stringOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

func timeOr(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func buildGroups(filtered, pageViews []view, startIndex, pageEnd int) []Group {
	type bucket struct {
		itemCount int
		minRN     int
		maxRN     int
		firstView view
	}
	buckets := make(map[string]*bucket)
	order := make([]string, 0)
	for i, v := range filtered {
		key := v.period.Key
		b, ok := buckets[key]
		if !ok {
			b = &bucket{firstView: v, minRN: i + 1}
			buckets[key] = b
			order = append(order, key)
		}
		b.itemCount++
		b.maxRN = i + 1
	}

	pageItemCounts := make(map[string]int)
	firstPageViewByKey := make(map[string]view)
	pageOrder := make([]string, 0)
	for _, v := range pageViews {
		key := v.period.Key
		if _, ok := firstPageViewByKey[key]; !ok {
			firstPageViewByKey[key] = v
			pageOrder = append(pageOrder, key)
		}
		pageItemCounts[key]++
	}

	totalsByKey := make(map[string][]view)
	for _, v := range filtered {
		if v.included {
			totalsByKey[v.period.Key] = append(totalsByKey[v.period.Key], v)
		}
	}

	groups := make([]Group, 0, len(pageOrder))
	for _, key := range pageOrder {
		v := firstPageViewByKey[key]
		b := buckets[key]
		groups = append(groups, Group{
			Key:            key,
			Kind:           v.period.Kind,
			StartDate:      v.period.StartDate,
			EndDate:        v.period.EndDate,
			ItemCount:      b.itemCount,
			PageItemCount:  pageItemCounts[key],
			HasItemsBefore: b.minRN <= startIndex,
			HasItemsAfter:  b.maxRN >= pageEnd+1,
			Totals:         currencyTotalsFor(totalsByKey[key]),
		})
	}
	return groups
}

func currencyTotalsFor(views []view) []CurrencyTotals {
	type accum struct {
		inflow  decimal.Decimal
		outflow decimal.Decimal
	}
	byCurrency := make(map[string]*accum)
	for _, v := range views {
		if v.effective == nil {
			continue
		}
		a, ok := byCurrency[v.effective.CurrencyCode]
		if !ok {
			a = &accum{inflow: decimal.Zero, outflow: decimal.Zero}
			byCurrency[v.effective.CurrencyCode] = a
		}
		switch v.classification {
		case money.Inflow:
			a.inflow = a.inflow.Add(v.effective.Value.Abs())
		case money.Outflow:
			a.outflow = a.outflow.Add(v.effective.Value.Abs())
		}
	}
	currencies := make([]string, 0, len(byCurrency))
	for c := range byCurrency {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)

	totals := make([]CurrencyTotals, 0, len(currencies))
	for _, c := range currencies {
		a := byCurrency[c]
		totals = append(totals, CurrencyTotals{
			CurrencyCode: c,
			Inflow:       money.CanonicalDecimal(a.inflow),
			Outflow:      money.CanonicalDecimal(a.outflow),
			Balance:      money.CanonicalDecimal(a.inflow.Sub(a.outflow)),
		})
	}
	return totals
}

func buildOverallTotals(filtered []view) []CurrencyTotals {
	eligible := make([]view, 0, len(filtered))
	for _, v := range filtered {
		if v.included {
			eligible = append(eligible, v)
		}
	}
	return currencyTotalsFor(eligible)
}

// CategorySpending is one internal category's total outflow spend over a
// filtered period.
type CategorySpending struct {
	CategoryID   *string
	CategoryName string
	Amount       string
}

// SpendingByCategory sums each included, BRL-denominated outflow
// transaction matching filters, grouped by internal category (a nil
// CategoryID groups every uncategorized transaction together as "Sem
// categoria"), sorted by amount descending. It reuses the same
// filtering/eligibility machinery as Query, just aggregating by category
// instead of by date period, and — unlike Query — always restricts to
// outflow, since that is the only direction "spending" means here.
func SpendingByCategory(ctx context.Context, q Querier, filters Filters, timezone string) ([]CategorySpending, error) {
	allViews, err := fetchAllViews(ctx, q, QueryRequest{GroupBy: money.GroupNone, Filters: filters})
	if err != nil {
		return nil, err
	}
	bounds, err := computeDateBounds(filters, timezone)
	if err != nil {
		return nil, err
	}

	type accum struct {
		name   string
		amount decimal.Decimal
	}
	const uncategorizedKey = ""
	byCategory := make(map[string]*accum)
	order := make([]string, 0)
	for _, v := range allViews {
		if !matches(v, filters, bounds) || !v.included {
			continue
		}
		if v.classification != money.Outflow || v.effective == nil || v.effective.CurrencyCode != "BRL" {
			continue
		}
		key, name := uncategorizedKey, "Sem categoria"
		if v.row.categoryDecisionCategoryID != nil {
			key, name = *v.row.categoryDecisionCategoryID, stringOr(v.row.categoryName, "")
		}
		a, ok := byCategory[key]
		if !ok {
			a = &accum{name: name, amount: decimal.Zero}
			byCategory[key] = a
			order = append(order, key)
		}
		a.amount = a.amount.Add(v.effective.Value.Abs())
	}

	sort.SliceStable(order, func(i, j int) bool {
		return byCategory[order[i]].amount.GreaterThan(byCategory[order[j]].amount)
	})

	result := make([]CategorySpending, len(order))
	for i, key := range order {
		a := byCategory[key]
		item := CategorySpending{CategoryName: a.name, Amount: money.CanonicalDecimal(a.amount)}
		if key != uncategorizedKey {
			id := key
			item.CategoryID = &id
		}
		result[i] = item
	}
	return result, nil
}

func buildAvailableFilters(ctx context.Context, q Querier, allViews []view) (AvailableFilters, error) {
	type accountInfo struct {
		name        *string
		institution *string
	}
	seen := make(map[string]accountInfo)
	institutionSet := make(map[string]bool)
	for _, v := range allViews {
		r := v.row
		if _, ok := seen[r.accountID]; !ok {
			seen[r.accountID] = accountInfo{name: r.accountName, institution: r.accountInstitution}
		}
		if r.accountInstitution != nil {
			institutionSet[*r.accountInstitution] = true
		}
	}
	accountIDs := make([]string, 0, len(seen))
	for id := range seen {
		accountIDs = append(accountIDs, id)
	}
	sort.Slice(accountIDs, func(i, j int) bool {
		ni, nj := seen[accountIDs[i]].name, seen[accountIDs[j]].name
		if stringOr(ni, "") != stringOr(nj, "") {
			return stringOr(ni, "") < stringOr(nj, "")
		}
		return accountIDs[i] < accountIDs[j]
	})
	accounts := make([]AccountOption, 0, len(accountIDs))
	for _, id := range accountIDs {
		info := seen[id]
		accounts = append(accounts, AccountOption{ID: id, Name: info.name, Institution: info.institution})
	}

	institutions := make([]string, 0, len(institutionSet))
	for inst := range institutionSet {
		institutions = append(institutions, inst)
	}
	sort.Slice(institutions, func(i, j int) bool {
		return strings.ToLower(institutions[i]) < strings.ToLower(institutions[j])
	})

	cats, err := categories.List(ctx, q)
	if err != nil {
		return AvailableFilters{}, err
	}
	categoryOptions := make([]CategoryOption, len(cats))
	for i, c := range cats {
		categoryOptions[i] = CategoryOption{ID: c.ID, Name: c.Name, Kind: c.Kind, IsActive: c.IsActive}
	}

	return AvailableFilters{
		Accounts:     accounts,
		Institutions: institutions,
		Categories:   categoryOptions,
	}, nil
}
