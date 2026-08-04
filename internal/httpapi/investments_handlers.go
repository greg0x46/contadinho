package httpapi

import (
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
}

const investmentSelectColumns = `
	fi.id, fi.external_id, ds.display_name, fi.investment_type, fi.subtype, fi.name,
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

// netContributed sums financial_investment_transactions per investment_id as
// (aplicações - resgates), the amount actually put in net of what came back
// out. It's the only reliable stand-in for "valor aplicado" investments.amount
// isn't: for renda fixa holdings Pluggy's amount field turned out to be
// quantity × value (the current gross mark value), which produced false
// negative yields on healthy CDBs when the previous fallback naively did
// balance - amount instead of using the actual buy/sell history.
func netContributed(conn *sql.DB, investmentIDs []string) (map[string]decimal.Decimal, error) {
	if len(investmentIDs) == 0 {
		return map[string]decimal.Decimal{}, nil
	}
	placeholders := strings.Repeat("?,", len(investmentIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(investmentIDs))
	for i, id := range investmentIDs {
		args[i] = id
	}
	rows, err := conn.Query(`
		SELECT investment_id, movement_type, amount
		FROM financial_investment_transactions
		WHERE investment_id IN (`+placeholders+`) AND amount IS NOT NULL`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]decimal.Decimal{}
	for rows.Next() {
		var investmentID, amountRaw string
		var movementType sql.NullString
		if err := rows.Scan(&investmentID, &movementType, &amountRaw); err != nil {
			return nil, err
		}
		amount, err := decimal.NewFromString(amountRaw)
		if err != nil {
			continue
		}
		if movementType.String == "SELL" || movementType.String == "REDEMPTION" {
			amount = amount.Neg()
		}
		result[investmentID] = result[investmentID].Add(amount)
	}
	return result, rows.Err()
}

// applyYield fills YieldValue/YieldSource: Pluggy's own amount_profit when
// the provider sends it ("informado"), otherwise balance minus net
// contributed from the transaction history when that history exists
// ("calculado"), otherwise left nil rather than guessing.
func applyYield(d *investmentDTO, contributed map[string]decimal.Decimal) {
	if d.AmountProfit != nil {
		informado := "informado"
		d.YieldValue, d.YieldSource = d.AmountProfit, &informado
		return
	}
	net, hasHistory := contributed[d.ID]
	if !hasHistory || d.Balance == nil {
		return
	}
	balance, err := decimal.NewFromString(*d.Balance)
	if err != nil {
		return
	}
	value := money.CanonicalDecimal(balance.Sub(net))
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
		contributed, err := netContributed(conn, ids)
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
		contributed, err := netContributed(conn, []string{d.ID})
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
			SELECT id, external_id, movement_type, quantity, value, amount, occurred_at, trade_date
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
			if err := rows.Scan(&t.ID, &t.ExternalID, &t.MovementType, &t.Quantity, &t.Value, &t.Amount,
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
