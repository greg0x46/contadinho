package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
)

type investmentDTO struct {
	ID                   string     `json:"id"`
	ExternalID           string     `json:"external_id"`
	SourceDisplayName    *string    `json:"source_display_name"`
	InvestmentType       *string    `json:"investment_type"`
	Subtype              *string    `json:"subtype"`
	Name                 *string    `json:"name"`
	Balance              *string    `json:"balance"`
	CurrencyCode         *string    `json:"currency_code"`
	Quantity             *string    `json:"quantity"`
	Value                *string    `json:"value"`
	Amount               *string    `json:"amount"`
	AmountProfit         *string    `json:"amount_profit"`
	AmountWithdrawal     *string    `json:"amount_withdrawal"`
	Rate                 *string    `json:"rate"`
	RateType             *string    `json:"rate_type"`
	FixedAnnualRate      *string    `json:"fixed_annual_rate"`
	AnnualRate           *string    `json:"annual_rate"`
	LastTwelveMonthsRate *string    `json:"last_twelve_months_rate"`
	Issuer               *string    `json:"issuer"`
	DueDate              *time.Time `json:"due_date"`
	AsOfDate             *time.Time `json:"as_of_date"`
	ProviderUpdatedAt    *time.Time `json:"provider_updated_at"`
	YieldValue           *string    `json:"yield_value"`
	YieldSource          *string    `json:"yield_source"` // "informado" | "calculado"
	// YieldUnavailableReason is set only when YieldValue is nil:
	// "sem_historico" (no movements synced), "historico_incompleto" (the
	// movements that arrived cannot account for the position) or
	// "saldo_indisponivel" (the history nets, but the provider sent no
	// current balance to net it against).
	YieldUnavailableReason *string `json:"yield_unavailable_reason"`
}

const investmentSelectColumns = `
	fi.id, fi.external_id, ` + connectionNameColumn + `, fi.investment_type, fi.subtype, fi.name,
	fi.balance, fi.currency_code, fi.quantity, fi.value, fi.amount, fi.amount_profit,
	fi.amount_withdrawal, fi.rate, fi.rate_type, fi.fixed_annual_rate, fi.annual_rate,
	fi.last_twelve_months_rate, fi.issuer, fi.due_date, fi.as_of_date, fi.provider_updated_at`

func scanInvestment(row interface{ Scan(...any) error }) (investmentDTO, error) {
	var (
		d              investmentDTO
		dueDateRaw     sql.NullString
		asOfDateRaw    sql.NullString
		providerUpdRaw sql.NullString
	)
	if err := row.Scan(&d.ID, &d.ExternalID, &d.SourceDisplayName, &d.InvestmentType, &d.Subtype, &d.Name,
		&d.Balance, &d.CurrencyCode, &d.Quantity, &d.Value, &d.Amount, &d.AmountProfit,
		&d.AmountWithdrawal, &d.Rate, &d.RateType, &d.FixedAnnualRate, &d.AnnualRate,
		&d.LastTwelveMonthsRate, &d.Issuer, &dueDateRaw, &asOfDateRaw, &providerUpdRaw); err != nil {
		return investmentDTO{}, err
	}
	var err error
	if d.DueDate, err = db.ParseNullTime(dueDateRaw); err != nil {
		return investmentDTO{}, err
	}
	if d.AsOfDate, err = db.ParseNullTime(asOfDateRaw); err != nil {
		return investmentDTO{}, err
	}
	if d.ProviderUpdatedAt, err = db.ParseNullTime(providerUpdRaw); err != nil {
		return investmentDTO{}, err
	}
	return d, nil
}

// contributionInfo is what one investment's movement history says about the
// money put in, plus the evidence needed to decide whether that history can
// be trusted at all.
type contributionInfo struct {
	net       decimal.Decimal // aplicações - resgates
	movements int
	hasInflow bool
	// unknownDir counts movements whose direction could not be established,
	// unusableAmount those whose amount the provider did not send (or sent
	// unparseable). Both make the sum incomplete in a way netting cannot
	// recover from, so both have to be counted rather than skipped: a
	// movement that contributes nothing to net while still counting as
	// evidence of a complete history reports the whole balance as profit.
	unknownDir     int
	unusableAmount int
	boughtQty      decimal.Decimal
	soldQty        decimal.Decimal
}

// netContributed sums financial_investment_transactions per investment_id as
// (aplicações - resgates), the amount actually put in net of what came back
// out. It's the only reliable stand-in for "valor aplicado" investments.amount
// isn't: for renda fixa holdings Pluggy's amount field turned out to be
// quantity × value (the current gross mark value), which produced false
// negative yields on healthy CDBs when the previous fallback naively did
// balance - amount instead of using the actual buy/sell history.
//
// Direction comes from the normalized direction column, never from the
// provider's movement_type vocabulary. Deriving it here by string-matching
// movement types is what made INTEREST — a dividend leaving the investment —
// count as a contribution, reporting healthy positions as losses.
func netContributed(ctx context.Context, conn *sql.DB, investmentIDs []string) (map[string]contributionInfo, error) {
	if len(investmentIDs) == 0 {
		return map[string]contributionInfo{}, nil
	}
	in, args := db.InClause(investmentIDs)
	rows, err := conn.QueryContext(ctx, `
		SELECT investment_id, direction, quantity, amount
		FROM financial_investment_transactions
		WHERE investment_id IN (`+in+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]contributionInfo{}
	for rows.Next() {
		var investmentID string
		var direction, quantityRaw, amountRaw sql.NullString
		if err := rows.Scan(&investmentID, &direction, &quantityRaw, &amountRaw); err != nil {
			return nil, err
		}
		info := result[investmentID]
		info.movements++

		if !direction.Valid || (direction.String != "inflow" && direction.String != "outflow") {
			info.unknownDir++
			result[investmentID] = info
			continue
		}
		isInflow := direction.String == "inflow"
		if isInflow {
			info.hasInflow = true
		}
		amount, err := decimal.NewFromString(amountRaw.String)
		switch {
		case !amountRaw.Valid || err != nil:
			// The movement happened — it just cannot be added up. Counting it
			// keeps historyCovers honest; dropping it silently would leave a
			// history that looks complete and nets to less than it should.
			info.unusableAmount++
		case isInflow:
			info.net = info.net.Add(amount)
		default:
			info.net = info.net.Sub(amount)
		}
		if quantity, err := decimal.NewFromString(quantityRaw.String); quantityRaw.Valid && err == nil {
			if isInflow {
				info.boughtQty = info.boughtQty.Add(quantity)
			} else {
				info.soldQty = info.soldQty.Add(quantity)
			}
		}
		result[investmentID] = info
	}
	return result, rows.Err()
}

// historyCovers reports whether an investment's movement history is complete
// enough to derive a yield from.
//
// Pluggy only serves movements from the connection's own window, so a holding
// bought before the account was linked arrives with resgates and no aplicação
// behind them. Netting such a history reports the whole redemption —
// principal included — as profit.
//
// Two signals catch that. An all-outflow history is the obvious one. The
// subtler one is quantity: for renda variável, shares sold can never exceed
// shares bought, so an excess means purchases are missing even though some
// were captured (BBAS3 in production: 120 cotas sold against 80 bought,
// reported as R$ 1.286,84 of yield). The check is deliberately limited to
// EQUITY — a CDB's "quantity" accrues with interest, so redeeming slightly
// more units than were bought is exactly what a healthy one does.
//
// A movement the sum could not use — no direction to put it on a side, or no
// amount to put there — fails the same way for the same reason: what is
// missing from the netting is invisible in its result.
func historyCovers(info contributionInfo, investmentType *string) bool {
	if info.movements == 0 || !info.hasInflow || info.unknownDir > 0 || info.unusableAmount > 0 {
		return false
	}
	// Case-folded because the provider's investment "type" is free text, not
	// an enum this app validates — the same reason
	// normalizeInvestmentDirection folds movementType. A single "Equity"
	// would otherwise switch the guard off without anything reporting it.
	if investmentType != nil && strings.EqualFold(*investmentType, "EQUITY") &&
		info.soldQty.GreaterThan(info.boughtQty) {
		return false
	}
	return true
}

// applyYield fills YieldValue/YieldSource: Pluggy's own amount_profit when
// the provider sends it ("informado"), otherwise balance minus net
// contributed from the transaction history when that history is complete
// enough to net against ("calculado"). When neither holds it leaves the
// yield nil and says why, so the UI can distinguish a holding that simply
// has no movements yet from one whose history the provider only half sent.
func applyYield(d *investmentDTO, contributed map[string]contributionInfo) {
	if d.AmountProfit != nil {
		informado := "informado"
		d.YieldValue, d.YieldSource = d.AmountProfit, &informado
		return
	}
	info := contributed[d.ID]
	if info.movements == 0 {
		reason := "sem_historico"
		d.YieldUnavailableReason = &reason
		return
	}
	if !historyCovers(info, d.InvestmentType) {
		reason := "historico_incompleto"
		d.YieldUnavailableReason = &reason
		return
	}
	// The history is fine; there is just nothing to net it against. Saying so
	// beats the bare "Não disponível" the UI falls back to when no reason is
	// given — with a complete history on screen, silence here reads as a bug.
	unavailableBalance := func() {
		reason := "saldo_indisponivel"
		d.YieldUnavailableReason = &reason
	}
	if d.Balance == nil {
		unavailableBalance()
		return
	}
	balance, err := decimal.NewFromString(*d.Balance)
	if err != nil {
		unavailableBalance()
		return
	}
	value := money.CanonicalDecimal(balance.Sub(info.net))
	calculado := "calculado"
	d.YieldValue, d.YieldSource = &value, &calculado
}

func handleListInvestments(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.QueryContext(r.Context(), `
			SELECT `+investmentSelectColumns+`
			FROM financial_investments fi
			JOIN data_sources ds ON ds.id = fi.source_id
			ORDER BY fi.name, fi.id`)
		if err != nil {
			writeProblem(w, 503, "investments-unavailable", "Investimentos temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		defer rows.Close()

		result := []investmentDTO{}
		for rows.Next() {
			d, err := scanInvestment(rows)
			if err != nil {
				writeProblem(w, 503, "investments-unavailable", "Investimentos temporariamente indisponíveis", "Tente novamente em instantes.")
				return
			}
			result = append(result, d)
		}
		if err := rows.Err(); err != nil {
			writeProblem(w, 503, "investments-unavailable", "Investimentos temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}

		ids := make([]string, len(result))
		for i, d := range result {
			ids[i] = d.ID
		}
		contributed, err := netContributed(r.Context(), conn, ids)
		if err != nil {
			writeProblem(w, 503, "investments-unavailable", "Investimentos temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		for i := range result {
			applyYield(&result[i], contributed)
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleGetInvestment(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		row := conn.QueryRowContext(r.Context(), `
			SELECT `+investmentSelectColumns+`
			FROM financial_investments fi
			JOIN data_sources ds ON ds.id = fi.source_id
			WHERE fi.id = ?`, id)
		d, err := scanInvestment(row)
		if err == sql.ErrNoRows {
			writeProblem(w, 404, "investment-not-found", "Investimento não encontrado", "")
			return
		}
		if err != nil {
			writeProblem(w, 503, "investments-unavailable", "Investimentos temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		contributed, err := netContributed(r.Context(), conn, []string{d.ID})
		if err != nil {
			writeProblem(w, 503, "investments-unavailable", "Investimentos temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		applyYield(&d, contributed)
		writeJSON(w, http.StatusOK, d)
	}
}

type investmentTransactionDTO struct {
	ID           string     `json:"id"`
	ExternalID   string     `json:"external_id"`
	MovementType *string    `json:"movement_type"`
	Direction    *string    `json:"direction"` // "inflow" | "outflow"
	Quantity     *string    `json:"quantity"`
	Value        *string    `json:"value"`
	Amount       *string    `json:"amount"`
	OccurredAt   *time.Time `json:"occurred_at"`
	TradeDate    *time.Time `json:"trade_date"`
}

func handleListInvestmentTransactions(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		investmentID := r.PathValue("id")

		var exists bool
		if err := conn.QueryRowContext(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM financial_investments WHERE id = ?)`, investmentID,
		).Scan(&exists); err != nil {
			writeProblem(w, 503, "investments-unavailable", "Investimentos temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		if !exists {
			writeProblem(w, 404, "investment-not-found", "Investimento não encontrado", "")
			return
		}

		rows, err := conn.QueryContext(r.Context(), `
			SELECT id, external_id, movement_type, direction, quantity, value, amount, occurred_at, trade_date
			FROM financial_investment_transactions
			WHERE investment_id = ?
			ORDER BY occurred_at DESC, id DESC`, investmentID)
		if err != nil {
			writeProblem(w, 503, "investments-unavailable", "Investimentos temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		defer rows.Close()

		result := []investmentTransactionDTO{}
		for rows.Next() {
			var (
				t             investmentTransactionDTO
				occurredAtRaw sql.NullString
				tradeDateRaw  sql.NullString
			)
			if err := rows.Scan(&t.ID, &t.ExternalID, &t.MovementType, &t.Direction, &t.Quantity, &t.Value, &t.Amount,
				&occurredAtRaw, &tradeDateRaw); err != nil {
				writeProblem(w, 503, "investments-unavailable", "Investimentos temporariamente indisponíveis", "Tente novamente em instantes.")
				return
			}
			if t.OccurredAt, err = db.ParseNullTime(occurredAtRaw); err != nil {
				continue
			}
			if t.TradeDate, err = db.ParseNullTime(tradeDateRaw); err != nil {
				continue
			}
			result = append(result, t)
		}
		writeJSON(w, http.StatusOK, result)
	}
}
