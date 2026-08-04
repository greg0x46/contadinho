package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/money"
	"contadinho-go/internal/receivables"
)

type receivableDTO struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	TotalAmount            string    `json:"total_amount"`
	StartingReceivedAmount string    `json:"starting_received_amount"`
	ReceivedAmount         string    `json:"received_amount"`
	RemainingAmount        string    `json:"remaining_amount"`
	Status                 string    `json:"status"`
	LinkCount              int       `json:"link_count"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func receivableUnavailableProblem(w http.ResponseWriter) {
	writeProblem(w, 503, "receivables-unavailable", "Contas a receber temporariamente indisponíveis", "Tente novamente em instantes.")
}

func receivableNotFoundProblem(w http.ResponseWriter) {
	writeProblem(w, 404, "receivable-not-found", "Conta a receber não encontrada", "")
}

// summarizeReceivable mirrors summarize (debts_handlers.go): received/
// remaining/status are always recomputed from the receivable's links, never
// read off a stored column.
func summarizeReceivable(ctx context.Context, conn *sql.DB, rec receivables.Receivable) (receivableDTO, []receivables.Link, error) {
	links, err := receivables.Links(ctx, conn, rec.ID)
	if err != nil {
		return receivableDTO{}, nil, err
	}
	amounts := make([]decimal.Decimal, len(links))
	for i, l := range links {
		amt, err := receivables.LinkEffectiveAmount(ctx, conn, l.TransactionID)
		if err != nil {
			return receivableDTO{}, nil, err
		}
		amounts[i] = amt
	}
	received := receivables.ReceivedAmount(rec.StartingReceivedAmount, amounts)
	remaining := receivables.RemainingAmount(rec.TotalAmount, received)
	status := receivables.StatusFor(remaining)
	return receivableDTO{
		ID: rec.ID, Name: rec.Name,
		TotalAmount:            money.CanonicalDecimal(rec.TotalAmount),
		StartingReceivedAmount: money.CanonicalDecimal(rec.StartingReceivedAmount),
		ReceivedAmount:         money.CanonicalDecimal(received),
		RemainingAmount:        money.CanonicalDecimal(remaining),
		Status:                 string(status),
		LinkCount:              len(links),
		CreatedAt:              rec.CreatedAt,
		UpdatedAt:              rec.UpdatedAt,
	}, links, nil
}

func handleListReceivables(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := receivables.List(r.Context(), conn)
		if err != nil {
			receivableUnavailableProblem(w)
			return
		}
		dtos := make([]receivableDTO, len(list))
		for i, rec := range list {
			dto, _, err := summarizeReceivable(r.Context(), conn, rec)
			if err != nil {
				receivableUnavailableProblem(w)
				return
			}
			dtos[i] = dto
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

type receivableTotalDTO struct {
	RemainingReceivablesTotal string `json:"remaining_receivables_total"`
	TotalToReceive            string `json:"total_to_receive"`
	CurrencyCode              string `json:"currency_code"`
}

// handleReceivableTotalToReceive sums the remaining_amount of every open
// Receivable — unlike handleDebtTotalOwed, there's no open-finance
// "future installments" counterpart to fold in here: a receivable has no
// schedule of its own, and future inflows aren't reported as PENDING
// transactions the way future credit-card installments are.
func handleReceivableTotalToReceive(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		list, err := receivables.List(ctx, conn)
		if err != nil {
			receivableUnavailableProblem(w)
			return
		}
		remainingTotal := decimal.Zero
		for _, rec := range list {
			dto, _, err := summarizeReceivable(ctx, conn, rec)
			if err != nil {
				receivableUnavailableProblem(w)
				return
			}
			if dto.Status != string(receivables.StatusOpen) {
				continue
			}
			remaining, err := decimal.NewFromString(dto.RemainingAmount)
			if err != nil {
				receivableUnavailableProblem(w)
				return
			}
			remainingTotal = remainingTotal.Add(remaining)
		}
		writeJSON(w, http.StatusOK, receivableTotalDTO{
			RemainingReceivablesTotal: money.CanonicalDecimal(remainingTotal),
			TotalToReceive:            money.CanonicalDecimal(remainingTotal),
			CurrencyCode:              "BRL",
		})
	}
}

type receivableCreateRequest struct {
	Name                   string           `json:"name"`
	TotalAmount            decimal.Decimal  `json:"total_amount"`
	InitialRemainingAmount *decimal.Decimal `json:"initial_remaining_amount"`
}

func handleCreateReceivable(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req receivableCreateRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-receivable", "Conta a receber inválida", "Revise os campos enviados.")
			return
		}
		if req.Name == "" || !req.TotalAmount.IsPositive() {
			writeProblem(w, 422, "invalid-receivable", "Conta a receber inválida", "Revise os campos enviados.")
			return
		}
		initial := req.TotalAmount
		if req.InitialRemainingAmount != nil {
			if req.InitialRemainingAmount.IsNegative() || req.InitialRemainingAmount.GreaterThan(req.TotalAmount) {
				writeProblem(w, 422, "invalid-receivable", "Conta a receber inválida", "Revise os campos enviados.")
				return
			}
			initial = *req.InitialRemainingAmount
		}
		rec, err := receivables.Create(r.Context(), conn, req.Name, req.TotalAmount, initial)
		if err != nil {
			receivableUnavailableProblem(w)
			return
		}
		dto, _, err := summarizeReceivable(r.Context(), conn, rec)
		if err != nil {
			receivableUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusCreated, dto)
	}
}

type eligibleReceivableTransactionMoneyDTO struct {
	Value        string `json:"value"`
	CurrencyCode string `json:"currency_code"`
}

type eligibleReceivableTransactionDTO struct {
	ID             string                                `json:"id"`
	OccurredAt     *time.Time                            `json:"occurred_at"`
	Description    *string                               `json:"description"`
	AccountName    *string                               `json:"account_name"`
	EffectiveMoney eligibleReceivableTransactionMoneyDTO `json:"effective_money"`
}

func handleListEligibleReceivableTransactions(conn *sql.DB) http.HandlerFunc {
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
		rows, err := receivables.ListEligibleTransactions(r.Context(), conn, search, limit)
		if err != nil {
			writeProblem(w, 503, "receivable-eligible-transactions-unavailable", "Contas a receber temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		dtos := make([]eligibleReceivableTransactionDTO, len(rows))
		for i, row := range rows {
			dtos[i] = eligibleReceivableTransactionDTO{
				ID: row.ID, OccurredAt: row.OccurredAt, Description: row.Description, AccountName: row.AccountName,
				EffectiveMoney: eligibleReceivableTransactionMoneyDTO{
					Value:        money.CanonicalDecimal(row.EffectiveMoney.Value.Abs()),
					CurrencyCode: row.EffectiveMoney.CurrencyCode,
				},
			}
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

type linkedReceivableTransactionDTO struct {
	ID            string     `json:"id"`
	TransactionID string     `json:"transaction_id"`
	OccurredAt    *time.Time `json:"occurred_at"`
	Description   *string    `json:"description"`
	LinkedAmount  string     `json:"linked_amount"`
	CurrentAmount string     `json:"current_amount"`
	LinkedAt      time.Time  `json:"linked_at"`
}

type receivableDetailDTO struct {
	receivableDTO
	Links []linkedReceivableTransactionDTO `json:"links"`
}

func handleGetReceivable(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		rec, err := receivables.Get(r.Context(), conn, id)
		if errors.Is(err, receivables.ErrNotFound) {
			receivableNotFoundProblem(w)
			return
		}
		if err != nil {
			receivableUnavailableProblem(w)
			return
		}
		dto, links, err := summarizeReceivable(r.Context(), conn, rec)
		if err != nil {
			receivableUnavailableProblem(w)
			return
		}
		linkDTOs := make([]linkedReceivableTransactionDTO, len(links))
		for i, l := range links {
			summary, err := receivables.TransactionSummaryFor(r.Context(), conn, l.TransactionID)
			if err != nil {
				receivableUnavailableProblem(w)
				return
			}
			current, err := receivables.LinkEffectiveAmount(r.Context(), conn, l.TransactionID)
			if err != nil {
				receivableUnavailableProblem(w)
				return
			}
			linkDTOs[i] = linkedReceivableTransactionDTO{
				ID: l.ID, TransactionID: l.TransactionID, OccurredAt: summary.OccurredAt, Description: summary.Description,
				LinkedAmount: money.CanonicalDecimal(l.LinkedAmount), CurrentAmount: money.CanonicalDecimal(current), LinkedAt: l.LinkedAt,
			}
		}
		writeJSON(w, http.StatusOK, receivableDetailDTO{receivableDTO: dto, Links: linkDTOs})
	}
}

type receivableUpdateRequest struct {
	Name        string          `json:"name"`
	TotalAmount decimal.Decimal `json:"total_amount"`
}

func handleUpdateReceivable(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req receivableUpdateRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-receivable", "Conta a receber inválida", "Revise os campos enviados.")
			return
		}
		if req.Name == "" || !req.TotalAmount.IsPositive() {
			writeProblem(w, 422, "invalid-receivable", "Conta a receber inválida", "Revise os campos enviados.")
			return
		}
		rec, err := receivables.Update(r.Context(), conn, id, req.Name, req.TotalAmount)
		if errors.Is(err, receivables.ErrNotFound) {
			receivableNotFoundProblem(w)
			return
		}
		if err != nil {
			receivableUnavailableProblem(w)
			return
		}
		dto, _, err := summarizeReceivable(r.Context(), conn, rec)
		if err != nil {
			receivableUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusOK, dto)
	}
}

func handleDeleteReceivable(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := receivables.Delete(r.Context(), conn, id); errors.Is(err, receivables.ErrNotFound) {
			receivableNotFoundProblem(w)
			return
		} else if err != nil {
			receivableUnavailableProblem(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

var receivableIneligibilityDetail = map[receivables.LinkIneligibilityReason]string{
	receivables.ReasonIgnored:        "A transação está marcada como ignorada.",
	receivables.ReasonNotInflow:      "A transação não é uma entrada (crédito).",
	receivables.ReasonMissingBRLPair: "A transação não possui um valor efetivo seguro em reais.",
}

type receivableLinkCreateRequest struct {
	TransactionID string `json:"transaction_id"`
}

type receivableLinkDTO struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	LinkedAmount  string    `json:"linked_amount"`
	LinkedAt      time.Time `json:"linked_at"`
}

func handleCreateReceivableLink(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		receivableID := r.PathValue("id")
		var req receivableLinkCreateRequest
		if err := decodeStrict(r, &req); err != nil || req.TransactionID == "" {
			writeProblem(w, 422, "invalid-receivable", "Conta a receber inválida", "Revise os campos enviados.")
			return
		}
		result, err := receivables.CreateLink(r.Context(), conn, receivableID, req.TransactionID)
		if err != nil {
			writeProblem(w, 503, "receivable-link-unavailable", "Contas a receber temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		switch result.Status {
		case receivables.StatusReceivableNotFound:
			receivableNotFoundProblem(w)
		case receivables.StatusTransactionNotFound:
			writeProblem(w, 404, "receivable-transaction-not-found", "Transação não encontrada", "")
		case receivables.StatusConflict:
			writeProblem(w, 409, "receivable-transaction-already-linked", "Transação já vinculada", "A transação já está vinculada a uma conta a receber.")
		case receivables.StatusIneligible:
			detail := receivableIneligibilityDetail[*result.Reason]
			writeProblem(w, 422, "receivable-transaction-ineligible", "Transação não elegível para vínculo", detail)
		default:
			writeJSON(w, http.StatusCreated, receivableLinkDTO{
				ID: result.Link.ID, TransactionID: result.Link.TransactionID,
				LinkedAmount: money.CanonicalDecimal(result.Link.LinkedAmount), LinkedAt: result.Link.LinkedAt,
			})
		}
	}
}

func handleDeleteReceivableLink(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		receivableID, linkID := r.PathValue("id"), r.PathValue("linkId")
		found, err := receivables.DeleteLink(r.Context(), conn, receivableID, linkID)
		if err != nil {
			receivableUnavailableProblem(w)
			return
		}
		if !found {
			writeProblem(w, 404, "receivable-link-not-found", "Vínculo não encontrado", "")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
