package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
	"contadinho-go/internal/payables"
)

type payableDTO struct {
	ID                    string    `json:"id"`
	Kind                  string    `json:"kind"`
	Name                  string    `json:"name"`
	TotalAmount           string    `json:"total_amount"`
	StartingSettledAmount string    `json:"starting_settled_amount"`
	SettledAmount         string    `json:"settled_amount"`
	RemainingAmount       string    `json:"remaining_amount"`
	Status                string    `json:"status"`
	LinkCount             int       `json:"link_count"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func payableUnavailableProblem(w http.ResponseWriter) {
	writeProblem(w, 503, "payables-unavailable", "Pendências temporariamente indisponíveis", "Tente novamente em instantes.")
}

// summarizePayable mirrors _to_response's math: settled/remaining/status
// are always recomputed from the payable's links, never read off a stored
// column.
func summarizePayable(ctx context.Context, conn *sql.DB, p payables.Payable) (payableDTO, []payables.Link, error) {
	links, err := payables.Links(ctx, conn, p.ID)
	if err != nil {
		return payableDTO{}, nil, err
	}
	amounts := make([]decimal.Decimal, len(links))
	for i, l := range links {
		amt, err := payables.LinkEffectiveAmount(ctx, conn, l.TransactionID)
		if err != nil {
			return payableDTO{}, nil, err
		}
		amounts[i] = amt
	}
	settled := payables.SettledAmount(p.StartingSettledAmount, amounts)
	remaining := payables.RemainingAmount(p.TotalAmount, settled)
	status := payables.StatusFor(remaining)
	return payableDTO{
		ID: p.ID, Kind: string(p.Kind), Name: p.Name,
		TotalAmount:           money.CanonicalDecimal(p.TotalAmount),
		StartingSettledAmount: money.CanonicalDecimal(p.StartingSettledAmount),
		SettledAmount:         money.CanonicalDecimal(settled),
		RemainingAmount:       money.CanonicalDecimal(remaining),
		Status:                string(status),
		LinkCount:             len(links),
		CreatedAt:             p.CreatedAt,
		UpdatedAt:             p.UpdatedAt,
	}, links, nil
}

// parseKindFilter reads an optional ?kind= query param, defaulting to nil
// (both kinds) when absent. An unrecognized value is treated as absent
// rather than erroring, matching parsePositiveInt's lenient style
// elsewhere in this package.
func parseKindFilter(r *http.Request) *payables.Kind {
	switch payables.Kind(r.URL.Query().Get("kind")) {
	case payables.KindDebt:
		k := payables.KindDebt
		return &k
	case payables.KindReceivable:
		k := payables.KindReceivable
		return &k
	default:
		return nil
	}
}

// requireKind reads a required ?kind= query param, writing a 422 problem
// and returning ok=false when it's missing or unrecognized.
func requireKind(w http.ResponseWriter, r *http.Request) (payables.Kind, bool) {
	switch k := payables.Kind(r.URL.Query().Get("kind")); k {
	case payables.KindDebt, payables.KindReceivable:
		return k, true
	default:
		writeProblem(w, 422, "invalid-payable-kind", "Tipo inválido", "Informe kind=debt ou kind=receivable.")
		return "", false
	}
}

func handleListPayables(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := payables.List(r.Context(), conn, parseKindFilter(r))
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		dtos := make([]payableDTO, len(list))
		for i, p := range list {
			dto, _, err := summarizePayable(r.Context(), conn, p)
			if err != nil {
				payableUnavailableProblem(w)
				return
			}
			dtos[i] = dto
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

type payableTotalOwedDTO struct {
	RemainingDebtsTotal     string `json:"remaining_debts_total"`
	FutureInstallmentsTotal string `json:"future_installments_total"`
	TotalOwed               string `json:"total_owed"`
	CurrencyCode            string `json:"currency_code"`
}

// handlePayableTotalOwed combines what's still owed on open debts (their
// remaining_amount, which has no schedule of its own — debts here are
// financing/loans that don't come through open finance) with the eligible
// credit-card transactions in the current bill cycle. Kept separate from
// handlePayableTotalToReceive since the credit-card bill-cycle folding-in
// below has no receivable counterpart — see handlePayableTotalToReceive's
// doc comment.
func handlePayableTotalOwed(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		kind := payables.KindDebt
		list, err := payables.List(ctx, conn, &kind)
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		remainingTotal := decimal.Zero
		for _, p := range list {
			dto, _, err := summarizePayable(ctx, conn, p)
			if err != nil {
				payableUnavailableProblem(w)
				return
			}
			if dto.Status != string(payables.StatusOpen) {
				continue
			}
			remaining, err := decimal.NewFromString(dto.RemainingAmount)
			if err != nil {
				payableUnavailableProblem(w)
				return
			}
			remainingTotal = remainingTotal.Add(remaining)
		}

		futureInstallmentsTotal, err := creditCardTransactionTotal(ctx, conn, "BRL")
		if err != nil {
			payableUnavailableProblem(w)
			return
		}

		writeJSON(w, http.StatusOK, payableTotalOwedDTO{
			RemainingDebtsTotal:     money.CanonicalDecimal(remainingTotal),
			FutureInstallmentsTotal: money.CanonicalDecimal(futureInstallmentsTotal),
			TotalOwed:               money.CanonicalDecimal(remainingTotal.Add(futureInstallmentsTotal)),
			CurrencyCode:            "BRL",
		})
	}
}

// creditCardTransactionTotal calculates the card amount from eligible transactions
// instead of using financial_accounts.balance. The provider balance does not
// carry this application's inclusion decisions and can therefore include
// payments or purchases that the user has ignored locally.
func creditCardTransactionTotal(ctx context.Context, conn *sql.DB, currencyCode string) (decimal.Decimal, error) {
	return creditCardTransactionTotalAt(ctx, conn, currencyCode, time.Now(), time.Local)
}

// creditCardTransactionTotalAt keeps the clock and location explicit so the cycle
// rule can be tested at calendar boundaries without changing the production
// behavior. time.Local is the application's process-configured timezone (for
// example, the TZ setting); it is intentionally used for local calendar days
// instead of comparing only UTC dates.
func creditCardTransactionTotalAt(ctx context.Context, conn *sql.DB, currencyCode string, now time.Time, loc *time.Location) (decimal.Decimal, error) {
	if loc == nil {
		return decimal.Zero, fmt.Errorf("credit card cycle timezone is nil")
	}

	bills, err := fetchCardBillClosingDates(ctx, conn, loc)
	if err != nil {
		return decimal.Zero, err
	}

	rows, err := conn.QueryContext(ctx, `
		SELECT id FROM financial_accounts
		WHERE account_type = 'CREDIT' AND currency_code = ?`, currencyCode)
	if err != nil {
		return decimal.Zero, fmt.Errorf("query credit card accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]string, 0)
	for rows.Next() {
		var accountID string
		if err := rows.Scan(&accountID); err != nil {
			return decimal.Zero, err
		}
		accounts = append(accounts, accountID)
	}
	if err := rows.Err(); err != nil {
		return decimal.Zero, err
	}

	total := decimal.Zero
	for _, accountID := range accounts {
		amount, err := consideredCardTransactionTotal(ctx, conn, accountID, currencyCode, bills, now, loc)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(amount)
	}
	return total, nil
}

type cardBillReference struct {
	closingDate *time.Time
}

type cardBillClosingDates struct {
	byAccount map[string][]time.Time
	byID      map[string]cardBillReference
}

func cardKey(accountID, billID string) string {
	return accountID + "\x00" + billID
}

func fetchCardBillClosingDates(ctx context.Context, conn *sql.DB, loc *time.Location) (cardBillClosingDates, error) {
	result := cardBillClosingDates{
		byAccount: make(map[string][]time.Time),
		byID:      make(map[string]cardBillReference),
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT account_id, id, external_id, closing_date
		FROM financial_bills`)
	if err != nil {
		return cardBillClosingDates{}, fmt.Errorf("query card bill closing dates: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var accountID, internalID, externalID string
		var closingDateRaw sql.NullString
		if err := rows.Scan(&accountID, &internalID, &externalID, &closingDateRaw); err != nil {
			return cardBillClosingDates{}, err
		}

		var closingDate *time.Time
		if closingDateRaw.Valid {
			parsed, err := db.ParseNullTime(closingDateRaw)
			if err != nil {
				return cardBillClosingDates{}, err
			}
			// closing_date is a provider calendar date stored in the
			// canonical UTC timestamp format. Preserve that date before
			// creating the boundary in the application's location; converting
			// midnight UTC directly to a negative-offset local date would move
			// the closing day backward.
			utcDate := parsed.UTC()
			boundary := time.Date(utcDate.Year(), utcDate.Month(), utcDate.Day(), 0, 0, 0, 0, loc)
			closingDate = &boundary
			result.byAccount[accountID] = append(result.byAccount[accountID], boundary)
		}

		reference := cardBillReference{closingDate: closingDate}
		result.byID[cardKey(accountID, externalID)] = reference
		// Pluggy's billId is the external id. Keeping the internal id too
		// makes the association safe for locally-created/test data without
		// changing the provider-facing rule.
		result.byID[cardKey(accountID, internalID)] = reference
	}
	if err := rows.Err(); err != nil {
		return cardBillClosingDates{}, err
	}

	for accountID := range result.byAccount {
		sort.Slice(result.byAccount[accountID], func(i, j int) bool {
			return result.byAccount[accountID][i].Before(result.byAccount[accountID][j])
		})
	}
	return result, nil
}

type creditCardCycle struct {
	start time.Time
	end   time.Time
}

func currentCreditCardCycle(bills cardBillClosingDates, accountID string, now time.Time, loc *time.Location) (creditCardCycle, bool) {
	closings := bills.byAccount[accountID]
	if len(closings) == 0 {
		return creditCardCycle{}, false
	}

	today := localDay(now, loc)
	var lastClosing time.Time
	var nextClosing time.Time
	for _, closing := range closings {
		if closing.After(today) {
			if !lastClosing.IsZero() {
				nextClosing = closing
			}
			break
		}
		lastClosing = closing
	}
	if lastClosing.IsZero() {
		// All available bills close in the future, so there is no
		// historical lower bound for the current cycle.
		return creditCardCycle{}, false
	}
	if nextClosing.IsZero() {
		nextClosing = inferredNextClosing(lastClosing, closings, loc)
	}
	if !nextClosing.After(lastClosing) {
		return creditCardCycle{}, false
	}
	return creditCardCycle{start: lastClosing, end: nextClosing}, true
}

func localDay(value time.Time, loc *time.Location) time.Time {
	local := value.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

func inferredNextClosing(lastClosing time.Time, closings []time.Time, loc *time.Location) time.Time {
	closingDay := lastClosing.Day()
	// A monthly closing on the 29th, 30th, or 31st can be represented by
	// the month's last day when that day does not exist. If earlier history
	// proves a larger nominal day, retain it for the next month.
	if closingDay == daysInMonth(lastClosing.Year(), lastClosing.Month(), loc) {
		for _, closing := range closings {
			if closing.Before(lastClosing) && closing.Day() > closingDay {
				closingDay = closing.Day()
			}
		}
	}

	nextMonth := time.Date(lastClosing.Year(), lastClosing.Month()+1, 1, 0, 0, 0, 0, loc)
	day := closingDay
	if maxDay := daysInMonth(nextMonth.Year(), nextMonth.Month(), loc); day > maxDay {
		day = maxDay
	}
	return time.Date(nextMonth.Year(), nextMonth.Month(), day, 0, 0, 0, 0, loc)
}

func daysInMonth(year int, month time.Month, loc *time.Location) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
}

type cardDebtTransaction struct {
	amount                  *decimal.Decimal
	amountInAccountCurrency *decimal.Decimal
	currencyCode            *string
	occurredAt              *time.Time
	providerStatus          *string
	movementType            *string
	creditCardMetadata      *string
	inclusionState          *string
}

// consideredCardTransactionTotal sums the local transactions that represent
// card cost in the current cycle. A card debit adds its absolute value to the
// positive debt indicator; a credit/refund subtracts its absolute value. An
// ignored bill payment never reaches this sign normalization.
func consideredCardTransactionTotal(
	ctx context.Context,
	conn *sql.DB,
	accountID, currencyCode string,
	bills cardBillClosingDates,
	now time.Time,
	loc *time.Location,
) (decimal.Decimal, error) {
	cycle, ok := currentCreditCardCycle(bills, accountID, now, loc)
	if !ok {
		// Without a reliable historical closing and a next boundary, the
		// conservative result is no locally calculated card amount. In
		// particular, never substitute due_date or billForecastDate here.
		return decimal.Zero, nil
	}

	transactions, err := fetchCardDebtTransactions(ctx, conn, accountID)
	if err != nil {
		return decimal.Zero, fmt.Errorf("query credit card transactions: %w", err)
	}

	accountCurrency := currencyCode
	total := decimal.Zero
	for _, transaction := range transactions {
		// A missing decision has the same domain meaning as considered. Only
		// an explicit ignored decision removes a transaction from the sum.
		if transaction.inclusionState != nil && *transaction.inclusionState != string(money.Considered) {
			continue
		}
		if !cardTransactionBelongsToCurrentCycle(accountID, transaction, bills, cycle) {
			continue
		}

		classification := money.Classify(transaction.movementType)
		effective := money.SelectEffectiveMoney(
			transaction.amountInAccountCurrency, &accountCurrency,
			transaction.amount, transaction.currencyCode,
		)
		included, _ := money.Eligibility(classification, transaction.providerStatus, effective, money.Considered)
		if !included || effective == nil || effective.CurrencyCode != currencyCode {
			continue
		}
		switch classification {
		case money.Outflow:
			total = total.Add(effective.Value.Abs())
		case money.Inflow:
			total = total.Sub(effective.Value.Abs())
		}
	}
	return total, nil
}

func cardTransactionIsInCycle(occurredAt *time.Time, cycle creditCardCycle) bool {
	return occurredAt != nil && !occurredAt.Before(cycle.start) && occurredAt.Before(cycle.end)
}

type cardTransactionMetadata struct {
	BillID           *string `json:"billId"`
	BillForecastDate *string `json:"billForecastDate"`
}

func cardTransactionBelongsToCurrentCycle(accountID string, transaction cardDebtTransaction, bills cardBillClosingDates, cycle creditCardCycle) bool {
	raw := transaction.creditCardMetadata
	if raw == nil {
		return cardTransactionIsInCycle(transaction.occurredAt, cycle)
	}
	var metadata cardTransactionMetadata
	if err := json.Unmarshal([]byte(*raw), &metadata); err != nil || metadata.BillID == nil || *metadata.BillID == "" {
		return cardTransactionIsInCycle(transaction.occurredAt, cycle)
	}

	reference, known := bills.byID[cardKey(accountID, *metadata.BillID)]
	if !known {
		// An unknown billId can refer to the provider's still-open bill,
		// which Pluggy does not return through the bills endpoint. The
		// occurrence date remains the only available classification signal.
		return cardTransactionIsInCycle(transaction.occurredAt, cycle)
	}
	if reference.closingDate == nil {
		// The association is known but its historical closing date is not;
		// do not risk counting it as a current-cycle transaction. This is
		// intentionally not a due_date fallback.
		return false
	}
	// The current-cycle bill is the next closing boundary, which is exclusive
	// for transactions classified only by occurred_at. A transaction already
	// assigned to the bill at that boundary belongs to that bill, even if the
	// provider's posting date is outside the cycle window.
	return reference.closingDate.After(cycle.start) && !reference.closingDate.After(cycle.end)
}

// billForecastDate is deliberately decoded above only to document the
// provider metadata shape. It identifies an expected month, not a closing
// boundary, so it never participates in the debt indicator's classification.

func fetchCardDebtTransactions(ctx context.Context, conn *sql.DB, accountID string) ([]cardDebtTransaction, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT ft.amount, ft.amount_in_account_currency, ft.currency_code,
		       ft.occurred_at, ft.provider_status, ft.movement_type, ft.credit_card_metadata,
		       tid.state
		FROM financial_transactions ft
		LEFT JOIN transaction_inclusion_decisions tid ON tid.transaction_id = ft.id
		WHERE ft.account_id = ?`, accountID)
	if err != nil {
		return nil, fmt.Errorf("query credit card transactions: %w", err)
	}
	defer rows.Close()

	result := make([]cardDebtTransaction, 0)
	for rows.Next() {
		var amountRaw, amountInAccountCurrencyRaw, currencyCodeRaw sql.NullString
		var occurredAtRaw, providerStatusRaw, movementTypeRaw, metadataRaw, inclusionStateRaw sql.NullString
		if err := rows.Scan(
			&amountRaw, &amountInAccountCurrencyRaw, &currencyCodeRaw,
			&occurredAtRaw,
			&providerStatusRaw, &movementTypeRaw, &metadataRaw, &inclusionStateRaw,
		); err != nil {
			return nil, err
		}
		amount, err := decimalFromNullString(amountRaw)
		if err != nil {
			return nil, err
		}
		amountInAccountCurrency, err := decimalFromNullString(amountInAccountCurrencyRaw)
		if err != nil {
			return nil, err
		}
		occurredAt, err := db.ParseNullTime(occurredAtRaw)
		if err != nil {
			return nil, err
		}
		result = append(result, cardDebtTransaction{
			amount:                  amount,
			amountInAccountCurrency: amountInAccountCurrency,
			currencyCode:            stringFromNullString(currencyCodeRaw),
			occurredAt:              occurredAt,
			providerStatus:          stringFromNullString(providerStatusRaw),
			movementType:            stringFromNullString(movementTypeRaw),
			creditCardMetadata:      stringFromNullString(metadataRaw),
			inclusionState:          stringFromNullString(inclusionStateRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func decimalFromNullString(value sql.NullString) (*decimal.Decimal, error) {
	if !value.Valid {
		return nil, nil
	}
	amount, err := decimal.NewFromString(value.String)
	if err != nil {
		return nil, fmt.Errorf("parse decimal %q: %w", value.String, err)
	}
	return &amount, nil
}

func stringFromNullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

type payableTotalToReceiveDTO struct {
	RemainingReceivablesTotal string `json:"remaining_receivables_total"`
	TotalToReceive            string `json:"total_to_receive"`
	CurrencyCode              string `json:"currency_code"`
}

// handlePayableTotalToReceive sums the remaining_amount of every open
// receivable — unlike handlePayableTotalOwed, there's no open-finance
// "future installments" counterpart to fold in here: a receivable has no
// schedule of its own, and future inflows aren't reported as PENDING
// transactions the way future credit-card installments are.
func handlePayableTotalToReceive(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		kind := payables.KindReceivable
		list, err := payables.List(ctx, conn, &kind)
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		remainingTotal := decimal.Zero
		for _, p := range list {
			dto, _, err := summarizePayable(ctx, conn, p)
			if err != nil {
				payableUnavailableProblem(w)
				return
			}
			if dto.Status != string(payables.StatusOpen) {
				continue
			}
			remaining, err := decimal.NewFromString(dto.RemainingAmount)
			if err != nil {
				payableUnavailableProblem(w)
				return
			}
			remainingTotal = remainingTotal.Add(remaining)
		}
		writeJSON(w, http.StatusOK, payableTotalToReceiveDTO{
			RemainingReceivablesTotal: money.CanonicalDecimal(remainingTotal),
			TotalToReceive:            money.CanonicalDecimal(remainingTotal),
			CurrencyCode:              "BRL",
		})
	}
}

type payableCreateRequest struct {
	Kind                   string           `json:"kind"`
	Name                   string           `json:"name"`
	TotalAmount            decimal.Decimal  `json:"total_amount"`
	InitialRemainingAmount *decimal.Decimal `json:"initial_remaining_amount"`
}

func invalidPayableProblem(w http.ResponseWriter) {
	writeProblem(w, 422, "invalid-payable", "Pendência inválida", "Revise os campos enviados.")
}

func handleCreatePayable(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req payableCreateRequest
		if err := decodeStrict(r, &req); err != nil {
			invalidPayableProblem(w)
			return
		}
		kind := payables.Kind(req.Kind)
		if (kind != payables.KindDebt && kind != payables.KindReceivable) || req.Name == "" || !req.TotalAmount.IsPositive() {
			invalidPayableProblem(w)
			return
		}
		initial := req.TotalAmount
		if req.InitialRemainingAmount != nil {
			if req.InitialRemainingAmount.IsNegative() || req.InitialRemainingAmount.GreaterThan(req.TotalAmount) {
				invalidPayableProblem(w)
				return
			}
			initial = *req.InitialRemainingAmount
		}
		p, err := payables.Create(r.Context(), conn, kind, req.Name, req.TotalAmount, initial)
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		dto, _, err := summarizePayable(r.Context(), conn, p)
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusCreated, dto)
	}
}

type eligibleTransactionMoneyDTO struct {
	Value        string `json:"value"`
	CurrencyCode string `json:"currency_code"`
}

type eligibleTransactionDTO struct {
	ID             string                      `json:"id"`
	OccurredAt     *time.Time                  `json:"occurred_at"`
	Description    *string                     `json:"description"`
	AccountName    *string                     `json:"account_name"`
	EffectiveMoney eligibleTransactionMoneyDTO `json:"effective_money"`
}

func handleListEligiblePayableTransactions(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind, ok := requireKind(w, r)
		if !ok {
			return
		}
		var search *string
		if v := r.URL.Query().Get("search"); v != "" {
			search = &v
		}
		limit := 20
		if v := r.URL.Query().Get("limit"); v != "" {
			if parsed, err := parsePositiveInt(v); err == nil && parsed >= 1 && parsed <= 50 {
				limit = parsed
			}
		}
		rows, err := payables.ListEligibleTransactions(r.Context(), conn, kind, search, limit)
		if err != nil {
			writeProblem(w, 503, "payable-eligible-transactions-unavailable", "Pendências temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		dtos := make([]eligibleTransactionDTO, len(rows))
		for i, row := range rows {
			dtos[i] = eligibleTransactionDTO{
				ID: row.ID, OccurredAt: row.OccurredAt, Description: row.Description, AccountName: row.AccountName,
				EffectiveMoney: eligibleTransactionMoneyDTO{
					Value:        money.CanonicalDecimal(row.EffectiveMoney.Value.Abs()),
					CurrencyCode: row.EffectiveMoney.CurrencyCode,
				},
			}
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

type linkedTransactionDTO struct {
	ID            string     `json:"id"`
	TransactionID string     `json:"transaction_id"`
	OccurredAt    *time.Time `json:"occurred_at"`
	Description   *string    `json:"description"`
	LinkedAmount  string     `json:"linked_amount"`
	CurrentAmount string     `json:"current_amount"`
	LinkedAt      time.Time  `json:"linked_at"`
}

type payableDetailDTO struct {
	payableDTO
	Links []linkedTransactionDTO `json:"links"`
}

func handleGetPayable(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		p, err := payables.Get(r.Context(), conn, id)
		if errors.Is(err, payables.ErrNotFound) {
			writeProblem(w, 404, "payable-not-found", "Pendência não encontrada", "")
			return
		}
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		dto, links, err := summarizePayable(r.Context(), conn, p)
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		linkDTOs := make([]linkedTransactionDTO, len(links))
		for i, l := range links {
			summary, err := payables.TransactionSummaryFor(r.Context(), conn, l.TransactionID)
			if err != nil {
				payableUnavailableProblem(w)
				return
			}
			current, err := payables.LinkEffectiveAmount(r.Context(), conn, l.TransactionID)
			if err != nil {
				payableUnavailableProblem(w)
				return
			}
			linkDTOs[i] = linkedTransactionDTO{
				ID: l.ID, TransactionID: l.TransactionID, OccurredAt: summary.OccurredAt, Description: summary.Description,
				LinkedAmount: money.CanonicalDecimal(l.LinkedAmount), CurrentAmount: money.CanonicalDecimal(current), LinkedAt: l.LinkedAt,
			}
		}
		writeJSON(w, http.StatusOK, payableDetailDTO{payableDTO: dto, Links: linkDTOs})
	}
}

type payableUpdateRequest struct {
	Name        string          `json:"name"`
	TotalAmount decimal.Decimal `json:"total_amount"`
}

func handleUpdatePayable(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req payableUpdateRequest
		if err := decodeStrict(r, &req); err != nil {
			invalidPayableProblem(w)
			return
		}
		if req.Name == "" || !req.TotalAmount.IsPositive() {
			invalidPayableProblem(w)
			return
		}
		p, err := payables.Update(r.Context(), conn, id, req.Name, req.TotalAmount)
		if errors.Is(err, payables.ErrNotFound) {
			writeProblem(w, 404, "payable-not-found", "Pendência não encontrada", "")
			return
		}
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		dto, _, err := summarizePayable(r.Context(), conn, p)
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusOK, dto)
	}
}

func handleDeletePayable(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := payables.Delete(r.Context(), conn, id); errors.Is(err, payables.ErrNotFound) {
			writeProblem(w, 404, "payable-not-found", "Pendência não encontrada", "")
			return
		} else if err != nil {
			payableUnavailableProblem(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

var payableIneligibilityDetail = map[payables.LinkIneligibilityReason]string{
	payables.ReasonIgnored:        "A transação está marcada como ignorada.",
	payables.ReasonNotOutflow:     "A transação não é uma saída (débito).",
	payables.ReasonNotInflow:      "A transação não é uma entrada (crédito).",
	payables.ReasonMissingBRLPair: "A transação não possui um valor efetivo seguro em reais.",
}

type payableLinkCreateRequest struct {
	TransactionID string `json:"transaction_id"`
}

type payableLinkDTO struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	LinkedAmount  string    `json:"linked_amount"`
	LinkedAt      time.Time `json:"linked_at"`
}

func handleCreatePayableLink(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payableID := r.PathValue("id")
		var req payableLinkCreateRequest
		if err := decodeStrict(r, &req); err != nil || req.TransactionID == "" {
			invalidPayableProblem(w)
			return
		}
		p, err := payables.Get(r.Context(), conn, payableID)
		if errors.Is(err, payables.ErrNotFound) {
			writeProblem(w, 404, "payable-not-found", "Pendência não encontrada", "")
			return
		}
		if err != nil {
			writeProblem(w, 503, "payable-link-unavailable", "Pendências temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		result, err := payables.CreateLink(r.Context(), conn, p.Kind, payableID, req.TransactionID)
		if err != nil {
			writeProblem(w, 503, "payable-link-unavailable", "Pendências temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		switch result.Status {
		case payables.StatusPayableNotFound:
			writeProblem(w, 404, "payable-not-found", "Pendência não encontrada", "")
		case payables.StatusTransactionNotFound:
			writeProblem(w, 404, "payable-transaction-not-found", "Transação não encontrada", "")
		case payables.StatusConflict:
			writeProblem(w, 409, "payable-transaction-already-linked", "Transação já vinculada", "A transação já está vinculada a uma pendência.")
		case payables.StatusIneligible:
			detail := payableIneligibilityDetail[*result.Reason]
			writeProblem(w, 422, "payable-transaction-ineligible", "Transação não elegível para vínculo", detail)
		default:
			writeJSON(w, http.StatusCreated, payableLinkDTO{
				ID: result.Link.ID, TransactionID: result.Link.TransactionID,
				LinkedAmount: money.CanonicalDecimal(result.Link.LinkedAmount), LinkedAt: result.Link.LinkedAt,
			})
		}
	}
}

func handleDeletePayableLink(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payableID, linkID := r.PathValue("id"), r.PathValue("linkId")
		found, err := payables.DeleteLink(r.Context(), conn, payableID, linkID)
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		if !found {
			writeProblem(w, 404, "payable-link-not-found", "Vínculo não encontrado", "")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
