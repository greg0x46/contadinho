package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/debts"
	"contadinho-go/internal/money"
	"contadinho-go/internal/transactions"
)

type debtDTO struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	TotalAmount        string    `json:"total_amount"`
	StartingPaidAmount string    `json:"starting_paid_amount"`
	PaidAmount         string    `json:"paid_amount"`
	RemainingAmount    string    `json:"remaining_amount"`
	Status             string    `json:"status"`
	LinkCount          int       `json:"link_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func debtUnavailableProblem(w http.ResponseWriter) {
	writeProblem(w, 503, "debts-unavailable", "Dívidas temporariamente indisponíveis", "Tente novamente em instantes.")
}

func debtNotFoundProblem(w http.ResponseWriter) {
	writeProblem(w, 404, "debt-not-found", "Dívida não encontrada", "")
}

// summarize mirrors _to_response's math: paid/remaining/status are always
// recomputed from the debt's links, never read off a stored column.
func summarize(ctx context.Context, conn *sql.DB, d debts.Debt) (debtDTO, []debts.Link, error) {
	links, err := debts.Links(ctx, conn, d.ID)
	if err != nil {
		return debtDTO{}, nil, err
	}
	amounts := make([]decimal.Decimal, len(links))
	for i, l := range links {
		amt, err := debts.LinkEffectiveAmount(ctx, conn, l.TransactionID)
		if err != nil {
			return debtDTO{}, nil, err
		}
		amounts[i] = amt
	}
	paid := debts.PaidAmount(d.StartingPaidAmount, amounts)
	remaining := debts.RemainingAmount(d.TotalAmount, paid)
	status := debts.StatusFor(remaining)
	return debtDTO{
		ID: d.ID, Name: d.Name,
		TotalAmount:        money.CanonicalDecimal(d.TotalAmount),
		StartingPaidAmount: money.CanonicalDecimal(d.StartingPaidAmount),
		PaidAmount:         money.CanonicalDecimal(paid),
		RemainingAmount:    money.CanonicalDecimal(remaining),
		Status:             string(status),
		LinkCount:          len(links),
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
	}, links, nil
}

func handleListDebts(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := debts.List(r.Context(), conn)
		if err != nil {
			debtUnavailableProblem(w)
			return
		}
		dtos := make([]debtDTO, len(list))
		for i, d := range list {
			dto, _, err := summarize(r.Context(), conn, d)
			if err != nil {
				debtUnavailableProblem(w)
				return
			}
			dtos[i] = dto
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

type debtTotalOwedDTO struct {
	RemainingDebtsTotal     string `json:"remaining_debts_total"`
	FutureInstallmentsTotal string `json:"future_installments_total"`
	TotalOwed               string `json:"total_owed"`
	CurrencyCode            string `json:"currency_code"`
}

// handleDebtTotalOwed combines what's still owed on open Debts (their
// remaining_amount, which has no schedule of its own — debts here are
// financing/loans that don't come through open finance) with the future,
// not-yet-billed credit-card installments open finance already reports as
// separate PENDING transactions (each with its own future occurred_at). No
// period concept is needed: PENDING vs POSTED already answers "already
// happened or still owed" regardless of calendar month.
func handleDebtTotalOwed(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		list, err := debts.List(ctx, conn)
		if err != nil {
			debtUnavailableProblem(w)
			return
		}
		remainingTotal := decimal.Zero
		for _, d := range list {
			dto, _, err := summarize(ctx, conn, d)
			if err != nil {
				debtUnavailableProblem(w)
				return
			}
			if dto.Status != string(debts.StatusOpen) {
				continue
			}
			remaining, err := decimal.NewFromString(dto.RemainingAmount)
			if err != nil {
				debtUnavailableProblem(w)
				return
			}
			remainingTotal = remainingTotal.Add(remaining)
		}

		pendingStatus := "PENDING"
		result, err := transactions.Query(ctx, conn, transactions.QueryRequest{
			GroupBy: money.GroupNone,
			Filters: transactions.Filters{ProviderStatus: &pendingStatus},
		})
		if err != nil {
			debtUnavailableProblem(w)
			return
		}
		futureInstallmentsTotal := decimal.Zero
		for _, t := range result.Totals {
			if t.CurrencyCode != "BRL" {
				continue
			}
			outflow, err := decimal.NewFromString(t.Outflow)
			if err != nil {
				debtUnavailableProblem(w)
				return
			}
			futureInstallmentsTotal = outflow
		}

		writeJSON(w, http.StatusOK, debtTotalOwedDTO{
			RemainingDebtsTotal:     money.CanonicalDecimal(remainingTotal),
			FutureInstallmentsTotal: money.CanonicalDecimal(futureInstallmentsTotal),
			TotalOwed:               money.CanonicalDecimal(remainingTotal.Add(futureInstallmentsTotal)),
			CurrencyCode:            "BRL",
		})
	}
}

type debtCreateRequest struct {
	Name                   string           `json:"name"`
	TotalAmount            decimal.Decimal  `json:"total_amount"`
	InitialRemainingAmount *decimal.Decimal `json:"initial_remaining_amount"`
}

func handleCreateDebt(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req debtCreateRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-debt", "Dívida inválida", "Revise os campos enviados.")
			return
		}
		if req.Name == "" || !req.TotalAmount.IsPositive() {
			writeProblem(w, 422, "invalid-debt", "Dívida inválida", "Revise os campos enviados.")
			return
		}
		initial := req.TotalAmount
		if req.InitialRemainingAmount != nil {
			if req.InitialRemainingAmount.IsNegative() || req.InitialRemainingAmount.GreaterThan(req.TotalAmount) {
				writeProblem(w, 422, "invalid-debt", "Dívida inválida", "Revise os campos enviados.")
				return
			}
			initial = *req.InitialRemainingAmount
		}
		debt, err := debts.Create(r.Context(), conn, req.Name, req.TotalAmount, initial)
		if err != nil {
			debtUnavailableProblem(w)
			return
		}
		dto, _, err := summarize(r.Context(), conn, debt)
		if err != nil {
			debtUnavailableProblem(w)
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

func handleListEligibleTransactions(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		rows, err := debts.ListEligibleTransactions(r.Context(), conn, search, limit)
		if err != nil {
			writeProblem(w, 503, "debt-eligible-transactions-unavailable", "Dívidas temporariamente indisponíveis", "Tente novamente em instantes.")
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

type debtDetailDTO struct {
	debtDTO
	Links []linkedTransactionDTO `json:"links"`
}

func handleGetDebt(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		debt, err := debts.Get(r.Context(), conn, id)
		if errors.Is(err, debts.ErrNotFound) {
			debtNotFoundProblem(w)
			return
		}
		if err != nil {
			debtUnavailableProblem(w)
			return
		}
		dto, links, err := summarize(r.Context(), conn, debt)
		if err != nil {
			debtUnavailableProblem(w)
			return
		}
		linkDTOs := make([]linkedTransactionDTO, len(links))
		for i, l := range links {
			summary, err := debts.TransactionSummaryFor(r.Context(), conn, l.TransactionID)
			if err != nil {
				debtUnavailableProblem(w)
				return
			}
			current, err := debts.LinkEffectiveAmount(r.Context(), conn, l.TransactionID)
			if err != nil {
				debtUnavailableProblem(w)
				return
			}
			linkDTOs[i] = linkedTransactionDTO{
				ID: l.ID, TransactionID: l.TransactionID, OccurredAt: summary.OccurredAt, Description: summary.Description,
				LinkedAmount: money.CanonicalDecimal(l.LinkedAmount), CurrentAmount: money.CanonicalDecimal(current), LinkedAt: l.LinkedAt,
			}
		}
		writeJSON(w, http.StatusOK, debtDetailDTO{debtDTO: dto, Links: linkDTOs})
	}
}

type debtUpdateRequest struct {
	Name        string          `json:"name"`
	TotalAmount decimal.Decimal `json:"total_amount"`
}

func handleUpdateDebt(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req debtUpdateRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-debt", "Dívida inválida", "Revise os campos enviados.")
			return
		}
		if req.Name == "" || !req.TotalAmount.IsPositive() {
			writeProblem(w, 422, "invalid-debt", "Dívida inválida", "Revise os campos enviados.")
			return
		}
		debt, err := debts.Update(r.Context(), conn, id, req.Name, req.TotalAmount)
		if errors.Is(err, debts.ErrNotFound) {
			debtNotFoundProblem(w)
			return
		}
		if err != nil {
			debtUnavailableProblem(w)
			return
		}
		dto, _, err := summarize(r.Context(), conn, debt)
		if err != nil {
			debtUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusOK, dto)
	}
}

func handleDeleteDebt(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := debts.Delete(r.Context(), conn, id); errors.Is(err, debts.ErrNotFound) {
			debtNotFoundProblem(w)
			return
		} else if err != nil {
			debtUnavailableProblem(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

var ineligibilityDetail = map[debts.LinkIneligibilityReason]string{
	debts.ReasonIgnored:        "A transação está marcada como ignorada.",
	debts.ReasonNotOutflow:     "A transação não é uma saída (débito).",
	debts.ReasonMissingBRLPair: "A transação não possui um valor efetivo seguro em reais.",
}

type debtLinkCreateRequest struct {
	TransactionID string `json:"transaction_id"`
}

type debtLinkDTO struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	LinkedAmount  string    `json:"linked_amount"`
	LinkedAt      time.Time `json:"linked_at"`
}

func handleCreateDebtLink(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debtID := r.PathValue("id")
		var req debtLinkCreateRequest
		if err := decodeStrict(r, &req); err != nil || req.TransactionID == "" {
			writeProblem(w, 422, "invalid-debt", "Dívida inválida", "Revise os campos enviados.")
			return
		}
		result, err := debts.CreateLink(r.Context(), conn, debtID, req.TransactionID)
		if err != nil {
			writeProblem(w, 503, "debt-link-unavailable", "Dívidas temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		switch result.Status {
		case debts.StatusDebtNotFound:
			debtNotFoundProblem(w)
		case debts.StatusTransactionNotFound:
			writeProblem(w, 404, "debt-transaction-not-found", "Transação não encontrada", "")
		case debts.StatusConflict:
			writeProblem(w, 409, "debt-transaction-already-linked", "Transação já vinculada", "A transação já está vinculada a uma dívida.")
		case debts.StatusIneligible:
			detail := ineligibilityDetail[*result.Reason]
			writeProblem(w, 422, "debt-transaction-ineligible", "Transação não elegível para vínculo", detail)
		default:
			writeJSON(w, http.StatusCreated, debtLinkDTO{
				ID: result.Link.ID, TransactionID: result.Link.TransactionID,
				LinkedAmount: money.CanonicalDecimal(result.Link.LinkedAmount), LinkedAt: result.Link.LinkedAt,
			})
		}
	}
}

func handleDeleteDebtLink(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debtID, linkID := r.PathValue("id"), r.PathValue("linkId")
		found, err := debts.DeleteLink(r.Context(), conn, debtID, linkID)
		if err != nil {
			debtUnavailableProblem(w)
			return
		}
		if !found {
			writeProblem(w, 404, "debt-link-not-found", "Vínculo não encontrado", "")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
