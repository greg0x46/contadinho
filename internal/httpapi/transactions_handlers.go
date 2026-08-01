package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/categories"
	"contadinho-go/internal/money"
	"contadinho-go/internal/scenarios"
	"contadinho-go/internal/transactions"
)

// --- request DTOs, matching frontend/src/api/contracts.ts's TransactionQuery ---

type transactionFiltersRequest struct {
	DateFrom       *string `json:"date_from"`
	DateTo         *string `json:"date_to"`
	Description    *string `json:"description"`
	AccountID      *string `json:"account_id"`
	Institution    *string `json:"institution"`
	CategoryID     *string `json:"category_id"`
	Classification *string `json:"classification"`
	ProviderStatus *string `json:"provider_status"`
	AmountMin      *string `json:"amount_min"`
	AmountMax      *string `json:"amount_max"`
	Uncategorized  *bool   `json:"uncategorized"`
}

type transactionQueryRequest struct {
	Timezone string                    `json:"timezone"`
	GroupBy  string                    `json:"group_by"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	Filters  transactionFiltersRequest `json:"filters"`
}

func parseDateOnly(s string) (money.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return money.Date{}, err
	}
	y, m, d := t.Date()
	return money.Date{Year: y, Month: m, Day: d}, nil
}

// toFilters translates the wire request into transactions.Filters, reporting
// a validation problem (mirroring transaction_routes.py's checks) instead of
// running the query when the input is malformed.
func toFilters(req transactionFiltersRequest) (transactions.Filters, *Problem) {
	invalid := func(kind, title, detail string) *Problem {
		return &Problem{Type: "/problems/" + kind, Title: title, Status: 400, Detail: detail}
	}

	var f transactions.Filters
	if (req.DateFrom == nil) != (req.DateTo == nil) {
		return f, invalid("invalid-date-range", "Intervalo de datas inválido",
			"Informe as datas inicial e final juntas, ou remova ambas.")
	}
	if req.DateFrom != nil && req.DateTo != nil {
		from, err1 := parseDateOnly(*req.DateFrom)
		to, err2 := parseDateOnly(*req.DateTo)
		if err1 != nil || err2 != nil {
			return f, invalid("invalid-date-range", "Intervalo de datas inválido", "Datas em formato inválido.")
		}
		if from.String() > to.String() {
			return f, invalid("invalid-date-range", "Intervalo de datas inválido",
				"A data inicial não pode ser posterior à data final.")
		}
		f.DateFrom, f.DateTo = &from, &to
	}
	f.Description = req.Description
	f.AccountID = req.AccountID
	f.Institution = req.Institution
	f.CategoryID = req.CategoryID
	if req.Classification != nil {
		c := money.Classification(*req.Classification)
		f.Classification = &c
	}
	f.ProviderStatus = req.ProviderStatus
	if req.AmountMin != nil {
		v, err := decimal.NewFromString(*req.AmountMin)
		if err != nil {
			return f, invalid("invalid-amount-range", "Faixa de valor inválida", "Valor mínimo inválido.")
		}
		f.AmountMin = &v
	}
	if req.AmountMax != nil {
		v, err := decimal.NewFromString(*req.AmountMax)
		if err != nil {
			return f, invalid("invalid-amount-range", "Faixa de valor inválida", "Valor máximo inválido.")
		}
		f.AmountMax = &v
	}
	if f.AmountMin != nil && f.AmountMax != nil && f.AmountMin.GreaterThan(*f.AmountMax) {
		return f, invalid("invalid-amount-range", "Faixa de valor inválida",
			"O valor mínimo não pode ser maior que o valor máximo.")
	}
	if req.Uncategorized != nil {
		f.Uncategorized = *req.Uncategorized
	}
	return f, nil
}

// --- response DTOs ---

type accountSummaryDTO struct {
	ID           string  `json:"id"`
	Name         *string `json:"name"`
	Institution  *string `json:"institution"`
	CurrencyCode *string `json:"currency_code"`
}

type effectiveMoneyDTO struct {
	Value        string `json:"value"`
	CurrencyCode string `json:"currency_code"`
	Source       string `json:"source"`
}

type inclusionDTO struct {
	State     string     `json:"state"`
	ChangedAt *time.Time `json:"changed_at"`
	Origin    string     `json:"origin"`
	RuleName  *string    `json:"rule_name"`
}

type internalCategoryDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	IsActive  bool      `json:"is_active"`
	Origin    string    `json:"origin"`
	ChangedAt time.Time `json:"changed_at"`
}

type totalsEligibilityDTO struct {
	Included bool    `json:"included"`
	Reason   *string `json:"reason"`
}

type transactionItemDTO struct {
	ID                      string               `json:"id"`
	ExternalID              string               `json:"external_id"`
	OccurredAt              *time.Time           `json:"occurred_at"`
	Description             *string              `json:"description"`
	Account                 accountSummaryDTO    `json:"account"`
	SourceCategory          *string              `json:"source_category"`
	InternalCategory        *internalCategoryDTO `json:"internal_category"`
	MovementType            *string              `json:"movement_type"`
	ProviderStatus          *string              `json:"provider_status"`
	Classification          string               `json:"classification"`
	Amount                  *string              `json:"amount"`
	CurrencyCode            *string              `json:"currency_code"`
	AmountInAccountCurrency *string              `json:"amount_in_account_currency"`
	EffectiveMoney          *effectiveMoneyDTO   `json:"effective_money"`
	Inclusion               inclusionDTO         `json:"inclusion"`
	TotalsEligibility       totalsEligibilityDTO `json:"totals_eligibility"`
	GroupKey                string               `json:"group_key"`
}

func toItemDTO(item transactions.Item) transactionItemDTO {
	dto := transactionItemDTO{
		ID:             item.ID,
		ExternalID:     item.ExternalID,
		OccurredAt:     item.OccurredAt,
		Description:    item.Description,
		SourceCategory: item.SourceCategory,
		MovementType:   item.MovementType,
		ProviderStatus: item.ProviderStatus,
		Classification: string(item.Classification),
		Amount:         item.Amount,
		CurrencyCode:   item.CurrencyCode,
		Account: accountSummaryDTO{
			ID:           item.Account.ID,
			Name:         item.Account.Name,
			Institution:  item.Account.Institution,
			CurrencyCode: item.Account.CurrencyCode,
		},
		AmountInAccountCurrency: item.AmountInAccountCurrency,
		Inclusion: inclusionDTO{
			State:     string(item.Inclusion.State),
			ChangedAt: item.Inclusion.ChangedAt,
			Origin:    item.Inclusion.Origin,
			RuleName:  item.Inclusion.RuleName,
		},
		TotalsEligibility: totalsEligibilityDTO{Included: item.TotalsEligibility.Included},
		GroupKey:          item.GroupKey,
	}
	if item.TotalsEligibility.Reason != nil {
		r := string(*item.TotalsEligibility.Reason)
		dto.TotalsEligibility.Reason = &r
	}
	if item.EffectiveMoney != nil {
		dto.EffectiveMoney = &effectiveMoneyDTO{
			Value:        item.EffectiveMoney.Value,
			CurrencyCode: item.EffectiveMoney.CurrencyCode,
			Source:       string(item.EffectiveMoney.Source),
		}
	}
	if item.InternalCategory != nil {
		dto.InternalCategory = &internalCategoryDTO{
			ID:        item.InternalCategory.ID,
			Name:      item.InternalCategory.Name,
			Kind:      string(item.InternalCategory.Kind),
			IsActive:  item.InternalCategory.IsActive,
			Origin:    item.InternalCategory.Origin,
			ChangedAt: item.InternalCategory.ChangedAt,
		}
	}
	return dto
}

type currencyTotalsDTO struct {
	CurrencyCode string `json:"currency_code"`
	Inflow       string `json:"inflow"`
	Outflow      string `json:"outflow"`
	Balance      string `json:"balance"`
}

func toCurrencyTotalsDTO(t transactions.CurrencyTotals) currencyTotalsDTO {
	return currencyTotalsDTO{CurrencyCode: t.CurrencyCode, Inflow: t.Inflow, Outflow: t.Outflow, Balance: t.Balance}
}

type groupDTO struct {
	Key            string              `json:"key"`
	Kind           string              `json:"kind"`
	StartDate      *string             `json:"start_date"`
	EndDate        *string             `json:"end_date"`
	ItemCount      int                 `json:"item_count"`
	PageItemCount  int                 `json:"page_item_count"`
	HasItemsBefore bool                `json:"has_items_before"`
	HasItemsAfter  bool                `json:"has_items_after"`
	Totals         []currencyTotalsDTO `json:"totals"`
}

func dateStringPtr(d *money.Date) *string {
	if d == nil {
		return nil
	}
	s := d.String()
	return &s
}

func toGroupDTO(g transactions.Group) groupDTO {
	totals := make([]currencyTotalsDTO, len(g.Totals))
	for i, t := range g.Totals {
		totals[i] = toCurrencyTotalsDTO(t)
	}
	return groupDTO{
		Key:            g.Key,
		Kind:           string(g.Kind),
		StartDate:      dateStringPtr(g.StartDate),
		EndDate:        dateStringPtr(g.EndDate),
		ItemCount:      g.ItemCount,
		PageItemCount:  g.PageItemCount,
		HasItemsBefore: g.HasItemsBefore,
		HasItemsAfter:  g.HasItemsAfter,
		Totals:         totals,
	}
}

type pageDTO struct {
	Number     int `json:"number"`
	Size       int `json:"size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

type accountOptionDTO struct {
	ID          string  `json:"id"`
	Name        *string `json:"name"`
	Institution *string `json:"institution"`
}

type categoryOptionDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	IsActive bool   `json:"is_active"`
}

type availableFiltersDTO struct {
	Accounts     []accountOptionDTO  `json:"accounts"`
	Institutions []string            `json:"institutions"`
	Categories   []categoryOptionDTO `json:"categories"`
}

type transactionQueryResultDTO struct {
	ConfirmedAt      time.Time            `json:"confirmed_at"`
	StoredTotal      int                  `json:"stored_total"`
	Page             pageDTO              `json:"page"`
	Items            []transactionItemDTO `json:"items"`
	Totals           []currencyTotalsDTO  `json:"totals"`
	Groups           []groupDTO           `json:"groups"`
	AvailableFilters availableFiltersDTO  `json:"available_filters"`
}

func toResultDTO(result transactions.Result) transactionQueryResultDTO {
	items := make([]transactionItemDTO, len(result.Items))
	for i, item := range result.Items {
		items[i] = toItemDTO(item)
	}
	totals := make([]currencyTotalsDTO, len(result.Totals))
	for i, t := range result.Totals {
		totals[i] = toCurrencyTotalsDTO(t)
	}
	groups := make([]groupDTO, len(result.Groups))
	for i, g := range result.Groups {
		groups[i] = toGroupDTO(g)
	}
	accounts := make([]accountOptionDTO, len(result.AvailableFilters.Accounts))
	for i, a := range result.AvailableFilters.Accounts {
		accounts[i] = accountOptionDTO{ID: a.ID, Name: a.Name, Institution: a.Institution}
	}
	categoryOptions := make([]categoryOptionDTO, len(result.AvailableFilters.Categories))
	for i, c := range result.AvailableFilters.Categories {
		categoryOptions[i] = categoryOptionDTO{ID: c.ID, Name: c.Name, Kind: string(c.Kind), IsActive: c.IsActive}
	}
	return transactionQueryResultDTO{
		ConfirmedAt: result.ConfirmedAt,
		StoredTotal: result.StoredTotal,
		Page: pageDTO{
			Number: result.Page.Number, Size: result.Page.Size,
			TotalItems: result.Page.TotalItems, TotalPages: result.Page.TotalPages,
		},
		Items:  items,
		Totals: totals,
		Groups: groups,
		AvailableFilters: availableFiltersDTO{
			Accounts:     accounts,
			Institutions: result.AvailableFilters.Institutions,
			Categories:   categoryOptions,
		},
	}
}

func handleQueryTransactions(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req transactionQueryRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-transaction-query", "Consulta de transações inválida", "Revise os campos enviados.")
			return
		}
		filters, problem := toFilters(req.Filters)
		if problem != nil {
			writeProblem(w, problem.Status, problem.Type[len("/problems/"):], problem.Title, problem.Detail)
			return
		}
		if _, err := time.LoadLocation(req.Timezone); err != nil || req.Timezone == "" {
			writeProblem(w, 400, "invalid-timezone", "Fuso horário inválido", "Informe um nome de fuso IANA reconhecido.")
			return
		}

		result, err := transactions.Query(r.Context(), db, transactions.QueryRequest{
			Timezone: req.Timezone,
			GroupBy:  money.GroupBy(req.GroupBy),
			Page:     req.Page,
			PageSize: req.PageSize,
			Filters:  filters,
		})
		if err != nil {
			writeProblem(w, 503, "transaction-query-unavailable", "Consulta temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		writeJSON(w, http.StatusOK, toResultDTO(result))
	}
}

type categorySpendingItemDTO struct {
	CategoryID   *string `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Amount       string  `json:"amount"`
	Source       string  `json:"source"`
}

type spendingByCategoryDTO struct {
	Month        string                    `json:"month"`
	CurrencyCode string                    `json:"currency_code"`
	Total        string                    `json:"total"`
	Items        []categorySpendingItemDTO `json:"items"`
}

func spendingByCategoryUnavailableProblem(w http.ResponseWriter) {
	writeProblem(w, 503, "spending-by-category-unavailable", "Gastos por categoria temporariamente indisponíveis", "Tente novamente em instantes.")
}

// dateWithinBounds reports whether t's calendar date (year/month/day, no
// time-of-day/timezone — matching how scenario_transactions.projected_at is
// stored) falls within [from, to] inclusive.
func dateWithinBounds(t time.Time, from, to money.Date) bool {
	d := money.Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
	notBefore := d.Year > from.Year ||
		(d.Year == from.Year && (d.Month > from.Month || (d.Month == from.Month && d.Day >= from.Day)))
	notAfter := d.Year < to.Year ||
		(d.Year == to.Year && (d.Month < to.Month || (d.Month == to.Month && d.Day <= to.Day)))
	return notBefore && notAfter
}

// projectedSpendingByCategory mirrors task 8's opt-in union: every
// scenario_transaction of scenarioID whose projected_at falls in [from, to],
// grouped by its free-text category (nil/empty groups as "Sem categoria",
// same convention as the real side) — never merged into a real category's
// total, always its own row tagged source="projetado", so the response can
// never conflate a hypothetical amount with a real one.
func projectedSpendingByCategory(ctx context.Context, db *sql.DB, scenarioID string, from, to money.Date) ([]categorySpendingItemDTO, error) {
	list, err := scenarios.ListScenarioTransactions(ctx, db, scenarioID)
	if err != nil {
		return nil, err
	}
	type accum struct {
		name   string
		amount decimal.Decimal
	}
	byCategory := make(map[string]*accum)
	var order []string
	for _, st := range list {
		if !dateWithinBounds(st.ProjectedAt, from, to) {
			continue
		}
		key, name := "", "Sem categoria"
		if st.Category != nil && *st.Category != "" {
			key, name = *st.Category, *st.Category
		}
		a, ok := byCategory[key]
		if !ok {
			a = &accum{name: name, amount: decimal.Zero}
			byCategory[key] = a
			order = append(order, key)
		}
		a.amount = a.amount.Add(st.Amount)
	}
	items := make([]categorySpendingItemDTO, len(order))
	for i, key := range order {
		a := byCategory[key]
		items[i] = categorySpendingItemDTO{CategoryName: a.name, Amount: money.CanonicalDecimal(a.amount), Source: "projetado"}
	}
	return items, nil
}

// handleSpendingByCategory sums the requesting client's current calendar
// month (in its own timezone, since "this month" depends on it) of outflow
// transactions grouped by internal category, powering the dashboard's
// spending widget. An optional scenario_id query param (task 8) unions in
// that scenario's projected installments due in the same period as
// separate, explicitly-tagged rows — never included unless asked for, and
// never merged into the real totals.
func handleSpendingByCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timezone := r.URL.Query().Get("timezone")
		loc, err := time.LoadLocation(timezone)
		if timezone == "" || err != nil {
			writeProblem(w, 400, "invalid-timezone", "Fuso horário inválido", "Informe um nome de fuso IANA reconhecido.")
			return
		}

		now := time.Now().In(loc)
		year, month, _ := now.Date()
		from := money.Date{Year: year, Month: month, Day: 1}
		lastDay := time.Date(year, month, 1, 0, 0, 0, 0, loc).AddDate(0, 1, -1)
		to := money.Date{Year: lastDay.Year(), Month: lastDay.Month(), Day: lastDay.Day()}

		items, err := transactions.SpendingByCategory(r.Context(), db, transactions.Filters{
			DateFrom: &from, DateTo: &to,
		}, timezone)
		if err != nil {
			spendingByCategoryUnavailableProblem(w)
			return
		}

		dtoItems := make([]categorySpendingItemDTO, len(items))
		for i, it := range items {
			dtoItems[i] = categorySpendingItemDTO{CategoryID: it.CategoryID, CategoryName: it.CategoryName, Amount: it.Amount, Source: "real"}
		}

		if scenarioID := r.URL.Query().Get("scenario_id"); scenarioID != "" {
			if _, err := scenarios.GetScenario(r.Context(), db, scenarioID); errors.Is(err, scenarios.ErrScenarioNotFound) {
				scenarioNotFoundProblem(w)
				return
			} else if err != nil {
				spendingByCategoryUnavailableProblem(w)
				return
			}
			projected, err := projectedSpendingByCategory(r.Context(), db, scenarioID, from, to)
			if err != nil {
				spendingByCategoryUnavailableProblem(w)
				return
			}
			dtoItems = append(dtoItems, projected...)
		}

		total := decimal.Zero
		for _, it := range dtoItems {
			amount, err := decimal.NewFromString(it.Amount)
			if err != nil {
				spendingByCategoryUnavailableProblem(w)
				return
			}
			total = total.Add(amount)
		}
		sort.SliceStable(dtoItems, func(i, j int) bool {
			ai, _ := decimal.NewFromString(dtoItems[i].Amount)
			aj, _ := decimal.NewFromString(dtoItems[j].Amount)
			return ai.GreaterThan(aj)
		})

		writeJSON(w, http.StatusOK, spendingByCategoryDTO{
			Month:        fmt.Sprintf("%04d-%02d", year, int(month)),
			CurrencyCode: "BRL",
			Total:        money.CanonicalDecimal(total),
			Items:        dtoItems,
		})
	}
}

type transactionInclusionRequest struct {
	State string `json:"state"`
}

type transactionInclusionResultDTO struct {
	TransactionID string     `json:"transaction_id"`
	State         string     `json:"state"`
	ChangedAt     *time.Time `json:"changed_at"`
}

func handleSetTransactionInclusion(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req transactionInclusionRequest
		if err := decodeStrict(r, &req); err != nil || (req.State != string(money.Considered) && req.State != string(money.Ignored)) {
			writeProblem(w, 422, "invalid-transaction-inclusion", "Decisão de inclusão inválida", "Revise o identificador e os campos enviados.")
			return
		}
		confirmation, err := transactions.SetInclusion(
			r.Context(), db, id, money.InclusionState(req.State), transactions.InclusionOriginManual, nil, nil, onIgnoredHook,
		)
		if errors.Is(err, transactions.ErrTransactionNotFound) {
			writeProblem(w, 404, "transaction-not-found", "Transação não encontrada", "")
			return
		}
		if err != nil {
			writeProblem(w, 503, "transaction-inclusion-unavailable", "Alteração temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		writeJSON(w, http.StatusOK, transactionInclusionResultDTO{
			TransactionID: confirmation.TransactionID,
			State:         string(confirmation.State),
			ChangedAt:     confirmation.ChangedAt,
		})
	}
}

type transactionCategoryRequest struct {
	CategoryID string `json:"category_id"`
}

type transactionCategoryResultDTO struct {
	TransactionID string    `json:"transaction_id"`
	CategoryID    string    `json:"category_id"`
	Origin        string    `json:"origin"`
	ChangedAt     time.Time `json:"changed_at"`
}

func handleSetTransactionCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req transactionCategoryRequest
		if err := decodeStrict(r, &req); err != nil || req.CategoryID == "" {
			writeProblem(w, 422, "invalid-transaction-category", "Atribuição de categoria inválida", "Revise o identificador e os campos enviados.")
			return
		}
		decision, err := categories.AssignManual(r.Context(), db, id, req.CategoryID)
		if errors.Is(err, categories.ErrTransactionNotFound) {
			writeProblem(w, 404, "transaction-not-found", "Transação não encontrada", "")
			return
		}
		if errors.Is(err, categories.ErrCategoryInvalid) {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "A categoria informada não existe ou está inativa.")
			return
		}
		if err != nil {
			writeProblem(w, 503, "transaction-category-unavailable", "Alteração temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		writeJSON(w, http.StatusOK, transactionCategoryResultDTO{
			TransactionID: decision.TransactionID,
			CategoryID:    decision.CategoryID,
			Origin:        "manual",
			ChangedAt:     decision.ChangedAt,
		})
	}
}
