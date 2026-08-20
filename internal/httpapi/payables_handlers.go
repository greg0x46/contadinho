package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

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

		futureInstallmentsTotal, err := payables.CreditCardTransactionTotal(ctx, conn, "BRL")
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
