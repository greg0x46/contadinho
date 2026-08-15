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
	"contadinho-go/internal/scenarios"
)

func scenariosUnavailableProblem(w http.ResponseWriter) {
	writeProblem(w, 503, "scenarios-unavailable", "Cenários temporariamente indisponíveis", "Tente novamente em instantes.")
}

func scenarioNotFoundProblem(w http.ResponseWriter) {
	writeProblem(w, 404, "scenario-not-found", "Cenário não encontrado", "")
}

func scenarioTransactionNotFoundProblem(w http.ResponseWriter) {
	writeProblem(w, 404, "scenario-transaction-not-found", "Parcela não encontrada", "")
}

func invalidScenarioProblem(w http.ResponseWriter, detail string) {
	writeProblem(w, 422, "invalid-scenario", "Cenário inválido", detail)
}

func invalidScenarioTransactionProblem(w http.ResponseWriter, detail string) {
	writeProblem(w, 422, "invalid-scenario-transaction", "Parcela inválida", detail)
}

const dateOnlyLayout = "2006-01-02"

type scenarioDTO struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	PayableID *string   `json:"payable_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func scenarioToDTO(s scenarios.Scenario) scenarioDTO {
	return scenarioDTO{
		ID: s.ID, Kind: string(s.Kind), Name: s.Name, PayableID: s.PayableID,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

type scenarioTransactionDTO struct {
	ID           string           `json:"id"`
	ScenarioID   string           `json:"scenario_id"`
	Description  string           `json:"description"`
	Amount       string           `json:"amount"`
	ProjectedAt  string           `json:"projected_at"`
	Category     *string          `json:"category"`
	Status       string           `json:"status"`
	Realizations []realizationDTO `json:"realizations"`
}

type realizationDTO struct {
	ID              string    `json:"id"`
	PayableLinkID   *string   `json:"payable_link_id"`
	AllocatedAmount string    `json:"allocated_amount"`
	CreatedAt       time.Time `json:"created_at"`
}

func realizationToDTO(r scenarios.ScenarioTransactionRealization) realizationDTO {
	return realizationDTO{
		ID:              r.ID,
		PayableLinkID:   r.PayableLinkID,
		AllocatedAmount: money.CanonicalDecimal(r.AllocatedAmount),
		CreatedAt:       r.CreatedAt,
	}
}

func realizationsToDTOs(list []scenarios.ScenarioTransactionRealization) []realizationDTO {
	dtos := make([]realizationDTO, len(list))
	for i, r := range list {
		dtos[i] = realizationToDTO(r)
	}
	return dtos
}

// todayUTC is "today" for ScenarioTransaction.Status purposes: midnight UTC,
// so an installment projected for today itself doesn't read as late just
// because the wall-clock time of day is already past midnight.
func todayUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// scenarioTransactionToDTO mirrors summarizePayable's role for payables:
// status is always recomputed here from today's date and realizedTotal,
// never read off a stored column.
func scenarioTransactionToDTO(st scenarios.ScenarioTransaction, today time.Time, realizedTotal decimal.Decimal, realizations []scenarios.ScenarioTransactionRealization) scenarioTransactionDTO {
	return scenarioTransactionDTO{
		ID: st.ID, ScenarioID: st.ScenarioID, Description: st.Description,
		Amount:       money.CanonicalDecimal(st.Amount),
		ProjectedAt:  st.ProjectedAt.Format(dateOnlyLayout),
		Category:     st.Category,
		Status:       string(st.Status(today, realizedTotal)),
		Realizations: realizationsToDTOs(realizations),
	}
}

// scenarioTransactionDTOFor fetches st's real realizedTotal (the sum of its
// scenario_transaction_realizations) before building the DTO — the one
// place every handler that returns a single scenario_transaction goes
// through, so Status always reflects real allocations.
func scenarioTransactionDTOFor(ctx context.Context, conn *sql.DB, st scenarios.ScenarioTransaction) (scenarioTransactionDTO, error) {
	realizations, err := scenarios.ListRealizationsForTransaction(ctx, conn, st.ID)
	if err != nil {
		return scenarioTransactionDTO{}, err
	}
	realizedTotal := decimal.Zero
	for _, r := range realizations {
		realizedTotal = realizedTotal.Add(r.AllocatedAmount)
	}
	return scenarioTransactionToDTO(st, todayUTC(), realizedTotal, realizations), nil
}

type scenarioDetailDTO struct {
	scenarioDTO
	Transactions         []scenarioTransactionDTO `json:"transactions"`
	AccumulatedDeviation string                   `json:"accumulated_deviation"`
}

// loadScenarioDetail also computes accumulated_deviation — Σ amount − Σ
// realizedTotal across every installment whose projected_at is on or before
// today. A positive value means the plan is behind (less was actually
// allocated than planned so far); negative means ahead. Installments not
// yet due don't count toward it, matching the spec's "ritmo necessário vs.
// ritmo real" framing — only what should already have happened factors
// into whether the plan is on track.
func loadScenarioDetail(w http.ResponseWriter, r *http.Request, conn *sql.DB, id string) (scenarioDetailDTO, bool) {
	s, err := scenarios.GetScenario(r.Context(), conn, id)
	if errors.Is(err, scenarios.ErrScenarioNotFound) {
		scenarioNotFoundProblem(w)
		return scenarioDetailDTO{}, false
	}
	if err != nil {
		scenariosUnavailableProblem(w)
		return scenarioDetailDTO{}, false
	}
	list, err := scenarios.ListScenarioTransactions(r.Context(), conn, s.ID)
	if err != nil {
		scenariosUnavailableProblem(w)
		return scenarioDetailDTO{}, false
	}
	today := todayUTC()
	deviation := decimal.Zero
	dtos := make([]scenarioTransactionDTO, len(list))
	for i, st := range list {
		realizations, err := scenarios.ListRealizationsForTransaction(r.Context(), conn, st.ID)
		if err != nil {
			scenariosUnavailableProblem(w)
			return scenarioDetailDTO{}, false
		}
		realizedTotal := decimal.Zero
		for _, real := range realizations {
			realizedTotal = realizedTotal.Add(real.AllocatedAmount)
		}
		dtos[i] = scenarioTransactionToDTO(st, today, realizedTotal, realizations)
		if !st.ProjectedAt.After(today) {
			deviation = deviation.Add(st.Amount.Sub(realizedTotal))
		}
	}
	return scenarioDetailDTO{
		scenarioDTO: scenarioToDTO(s), Transactions: dtos,
		AccumulatedDeviation: money.CanonicalDecimal(deviation),
	}, true
}

type scenarioCreateRequest struct {
	Name string `json:"name"`
}

// scenarioKindFor maps a payable's Kind to the scenario Kind attached to
// it — the only two scenario kinds that exist.
func scenarioKindFor(kind payables.Kind) scenarios.Kind {
	if kind == payables.KindReceivable {
		return scenarios.KindReceivablePlan
	}
	return scenarios.KindDebtPlan
}

// handleCreatePayableScenario creates a Scenario for the payable named by
// the {id} path value, with Kind derived from the payable's own Kind — the
// only creation entry point in this v1, since "what_if" scenarios (no
// payable_id) have no UI or use case yet.
func handleCreatePayableScenario(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payableID := r.PathValue("id")
		p, err := payables.Get(r.Context(), conn, payableID)
		if errors.Is(err, payables.ErrNotFound) {
			writeProblem(w, 404, "payable-not-found", "Pendência não encontrada", "")
			return
		} else if err != nil {
			payableUnavailableProblem(w)
			return
		}

		var req scenarioCreateRequest
		if err := decodeStrict(r, &req); err != nil || req.Name == "" {
			invalidScenarioProblem(w, "Informe um nome para o plano.")
			return
		}

		s, err := scenarios.CreateScenario(r.Context(), conn, scenarioKindFor(p.Kind), req.Name, &payableID)
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusCreated, scenarioDetailDTO{
			scenarioDTO: scenarioToDTO(s), Transactions: []scenarioTransactionDTO{}, AccumulatedDeviation: "0.00",
		})
	}
}

func handleListPayableScenarios(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payableID := r.PathValue("id")
		list, err := scenarios.ListScenariosByPayable(r.Context(), conn, payableID)
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		dtos := make([]scenarioDTO, len(list))
		for i, s := range list {
			dtos[i] = scenarioToDTO(s)
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

func handleGetScenario(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dto, ok := loadScenarioDetail(w, r, conn, r.PathValue("id"))
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, dto)
	}
}

func handleDeleteScenario(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := scenarios.DeleteScenario(r.Context(), conn, r.PathValue("id")); errors.Is(err, scenarios.ErrScenarioNotFound) {
			scenarioNotFoundProblem(w)
			return
		} else if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type scenarioTransactionWriteRequest struct {
	Description string          `json:"description"`
	Amount      decimal.Decimal `json:"amount"`
	ProjectedAt string          `json:"projected_at"`
	Category    *string         `json:"category"`
}

func (req scenarioTransactionWriteRequest) validate() (time.Time, bool) {
	if req.Description == "" || !req.Amount.IsPositive() {
		return time.Time{}, false
	}
	projectedAt, err := time.Parse(dateOnlyLayout, req.ProjectedAt)
	return projectedAt, err == nil
}

func handleCreateScenarioTransaction(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scenarioID := r.PathValue("id")
		if _, err := scenarios.GetScenario(r.Context(), conn, scenarioID); errors.Is(err, scenarios.ErrScenarioNotFound) {
			scenarioNotFoundProblem(w)
			return
		} else if err != nil {
			scenariosUnavailableProblem(w)
			return
		}

		var req scenarioTransactionWriteRequest
		if err := decodeStrict(r, &req); err != nil {
			invalidScenarioTransactionProblem(w, "Revise os campos enviados.")
			return
		}
		projectedAt, ok := req.validate()
		if !ok {
			invalidScenarioTransactionProblem(w, "Revise os campos enviados.")
			return
		}

		st, err := scenarios.CreateScenarioTransaction(r.Context(), conn, scenarioID, req.Description, req.Amount, projectedAt, req.Category)
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		dto, err := scenarioTransactionDTOFor(r.Context(), conn, st)
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusCreated, dto)
	}
}

func handleUpdateScenarioTransaction(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("transactionId")
		var req scenarioTransactionWriteRequest
		if err := decodeStrict(r, &req); err != nil {
			invalidScenarioTransactionProblem(w, "Revise os campos enviados.")
			return
		}
		projectedAt, ok := req.validate()
		if !ok {
			invalidScenarioTransactionProblem(w, "Revise os campos enviados.")
			return
		}

		st, err := scenarios.UpdateScenarioTransaction(r.Context(), conn, id, req.Description, req.Amount, projectedAt, req.Category)
		if errors.Is(err, scenarios.ErrTransactionNotFound) {
			scenarioTransactionNotFoundProblem(w)
			return
		}
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		dto, err := scenarioTransactionDTOFor(r.Context(), conn, st)
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusOK, dto)
	}
}

type generateInstallmentsRequest struct {
	Cadence           string           `json:"cadence"`
	Months            *int             `json:"months"`
	InstallmentAmount *decimal.Decimal `json:"installment_amount"`
	StartDate         *string          `json:"start_date"`
}

// handleGenerateInstallments lets the caller pick a cadence and either the
// number of installments or the value of each one — exactly one of the
// two, never both: given the scenario's payable remaining amount (computed
// the same way summarizePayable does), it creates that many
// scenario_transactions spaced by cadence, the last absorbing whatever
// division left over. It refuses to run on a scenario that already has
// installments — this button is meant to seed an empty plan once, not
// silently pile on duplicates; a caller that wants to regenerate deletes
// the existing ones first.
func handleGenerateInstallments(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scenarioID := r.PathValue("id")
		s, err := scenarios.GetScenario(r.Context(), conn, scenarioID)
		if errors.Is(err, scenarios.ErrScenarioNotFound) {
			scenarioNotFoundProblem(w)
			return
		}
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		if s.PayableID == nil {
			invalidScenarioProblem(w, "Este cenário não está associado a uma dívida ou conta a receber.")
			return
		}

		existing, err := scenarios.ListScenarioTransactions(r.Context(), conn, scenarioID)
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		if len(existing) > 0 {
			writeProblem(w, 409, "scenario-already-has-installments", "Plano já possui parcelas",
				"Exclua as parcelas existentes antes de gerar um novo conjunto.")
			return
		}

		var req generateInstallmentsRequest
		if err := decodeStrict(r, &req); err != nil {
			invalidScenarioTransactionProblem(w, "Revise os campos enviados.")
			return
		}
		cadence := scenarios.Cadence(req.Cadence)
		if !cadence.IsValid() {
			invalidScenarioTransactionProblem(w, "Informe uma cadência válida: mensal, semanal ou quinzenal.")
			return
		}
		hasMonths := req.Months != nil && *req.Months >= 1
		hasAmount := req.InstallmentAmount != nil && req.InstallmentAmount.IsPositive()
		if hasMonths == hasAmount {
			invalidScenarioTransactionProblem(w, "Informe o número de parcelas OU o valor de cada parcela — não os dois, nem nenhum.")
			return
		}
		startDate := time.Now().UTC()
		if req.StartDate != nil {
			parsed, err := time.Parse(dateOnlyLayout, *req.StartDate)
			if err != nil {
				invalidScenarioTransactionProblem(w, "Data inicial inválida.")
				return
			}
			startDate = parsed
		}

		remaining, ok := scenarioRemainingAmount(w, r, conn, s)
		if !ok {
			return
		}

		var drafts []scenarios.GeneratedInstallment
		if hasMonths {
			drafts, err = scenarios.GenerateInstallments(remaining, *req.Months, startDate, cadence)
		} else {
			drafts, err = scenarios.GenerateInstallmentsForAmount(remaining, *req.InstallmentAmount, startDate, cadence)
		}
		if err != nil {
			invalidScenarioTransactionProblem(w, "A dívida/conta a receber não possui valor restante para gerar parcelas.")
			return
		}
		created, err := scenarios.CreateGeneratedInstallments(r.Context(), conn, scenarioID, drafts)
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		dtos := make([]scenarioTransactionDTO, len(created))
		today := todayUTC()
		for i, st := range created {
			dtos[i] = scenarioTransactionToDTO(st, today, decimal.Zero, nil)
		}
		writeJSON(w, http.StatusCreated, dtos)
	}
}

// payableRemainingAmount recomputes a payable's remaining_amount the same
// way summarizePayable does — duplicated here rather than imported because
// summarizePayable returns a full payableDTO and this only needs the one
// number.
func payableRemainingAmount(ctx context.Context, conn *sql.DB, p payables.Payable) (decimal.Decimal, error) {
	links, err := payables.Links(ctx, conn, p.ID)
	if err != nil {
		return decimal.Decimal{}, err
	}
	amounts := make([]decimal.Decimal, len(links))
	for i, l := range links {
		amt, err := payables.LinkEffectiveAmount(ctx, conn, l.TransactionID)
		if err != nil {
			return decimal.Decimal{}, err
		}
		amounts[i] = amt
	}
	settled := payables.SettledAmount(p.StartingSettledAmount, amounts)
	return payables.RemainingAmount(p.TotalAmount, settled), nil
}

// scenarioRemainingAmount resolves s's remaining amount from its payable_id,
// writing the appropriate problem response and returning ok=false on any
// failure — shared by handleGenerateInstallments and
// handleReadjustInstallments, the two handlers that need "how much is
// left" regardless of which kind of scenario they're operating on.
func scenarioRemainingAmount(w http.ResponseWriter, r *http.Request, conn *sql.DB, s scenarios.Scenario) (decimal.Decimal, bool) {
	if s.PayableID == nil {
		invalidScenarioProblem(w, "Este cenário não está associado a uma dívida ou conta a receber.")
		return decimal.Decimal{}, false
	}
	p, err := payables.Get(r.Context(), conn, *s.PayableID)
	if errors.Is(err, payables.ErrNotFound) {
		writeProblem(w, 404, "payable-not-found", "Pendência não encontrada", "")
		return decimal.Decimal{}, false
	}
	if err != nil {
		payableUnavailableProblem(w)
		return decimal.Decimal{}, false
	}
	remaining, err := payableRemainingAmount(r.Context(), conn, p)
	if err != nil {
		payableUnavailableProblem(w)
		return decimal.Decimal{}, false
	}
	return remaining, true
}

type readjustRequest struct {
	Strategy string `json:"strategy"`
}

// handleReadjustInstallments mirrors "Reajustar parcelas restantes": it
// replaces every installment with no allocation at all with a fresh set
// that sums to the payable's current remaining amount minus what's already
// reserved by partially/fully allocated installments — those are never
// touched. See scenarios.Readjust for the two strategies.
func handleReadjustInstallments(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scenarioID := r.PathValue("id")
		s, err := scenarios.GetScenario(r.Context(), conn, scenarioID)
		if errors.Is(err, scenarios.ErrScenarioNotFound) {
			scenarioNotFoundProblem(w)
			return
		}
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		if s.PayableID == nil {
			invalidScenarioProblem(w, "Este cenário não está associado a uma dívida ou conta a receber.")
			return
		}

		var req readjustRequest
		if err := decodeStrict(r, &req); err != nil {
			invalidScenarioProblem(w, "Informe uma estratégia válida: abater_do_final ou redistribuir.")
			return
		}
		strategy := scenarios.ReadjustStrategy(req.Strategy)
		if strategy != scenarios.StrategyReduceTerm && strategy != scenarios.StrategyRedistribute {
			invalidScenarioProblem(w, "Informe uma estratégia válida: abater_do_final ou redistribuir.")
			return
		}

		remaining, ok := scenarioRemainingAmount(w, r, conn, s)
		if !ok {
			return
		}

		created, err := scenarios.Readjust(r.Context(), conn, scenarioID, remaining, strategy)
		switch {
		case errors.Is(err, scenarios.ErrNoAffectedInstallments):
			writeProblem(w, 422, "scenario-readjust-nothing-to-do", "Nada para reajustar",
				"Todas as parcelas já possuem alguma alocação.")
			return
		case errors.Is(err, scenarios.ErrNothingToRedistribute):
			writeProblem(w, 422, "scenario-readjust-no-balance", "Nada a redistribuir",
				"Não há saldo restante para gerar novas parcelas.")
			return
		case err != nil:
			scenariosUnavailableProblem(w)
			return
		}

		today := todayUTC()
		dtos := make([]scenarioTransactionDTO, len(created))
		for i, st := range created {
			dtos[i] = scenarioTransactionToDTO(st, today, decimal.Zero, nil)
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

func handleDeleteScenarioTransaction(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("transactionId")
		if err := scenarios.DeleteScenarioTransaction(r.Context(), conn, id); errors.Is(err, scenarios.ErrTransactionNotFound) {
			scenarioTransactionNotFoundProblem(w)
			return
		} else if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func realizationNotFoundProblem(w http.ResponseWriter) {
	writeProblem(w, 404, "scenario-realization-not-found", "Alocação não encontrada", "")
}

type realizationCreateRequest struct {
	PayableLinkID   string          `json:"payable_link_id"`
	AllocatedAmount decimal.Decimal `json:"allocated_amount"`
}

// handleCreateRealization mirrors "isso quita qual parcela planejada?"
// flow: allocating (part of) an existing payable_transaction_links row —
// created earlier through the ordinary "vincular transação" flow — to a
// planned installment. The link must belong to the scenario's own payable,
// so a link from an unrelated payable can never be allocated here. It
// otherwise places no cap on allocated_amount: over- and under-allocating
// are both valid outcomes Status already models (paga_a_mais/
// paga_parcialmente), and splitting one link across several installments
// or funding one installment from several links is exactly what this table
// is for.
func handleCreateRealization(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scenarioID, transactionID := r.PathValue("id"), r.PathValue("transactionId")

		s, err := scenarios.GetScenario(r.Context(), conn, scenarioID)
		if errors.Is(err, scenarios.ErrScenarioNotFound) {
			scenarioNotFoundProblem(w)
			return
		}
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		st, err := scenarios.GetScenarioTransaction(r.Context(), conn, transactionID)
		if errors.Is(err, scenarios.ErrTransactionNotFound) || (err == nil && st.ScenarioID != scenarioID) {
			scenarioTransactionNotFoundProblem(w)
			return
		}
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}

		var req realizationCreateRequest
		if err := decodeStrict(r, &req); err != nil || req.PayableLinkID == "" || !req.AllocatedAmount.IsPositive() {
			invalidScenarioTransactionProblem(w, "Informe um vínculo e um valor alocado válido (> 0).")
			return
		}
		if s.PayableID == nil {
			invalidScenarioTransactionProblem(w, "Este cenário não está associado a uma dívida ou conta a receber.")
			return
		}
		link, err := payables.GetLink(r.Context(), conn, req.PayableLinkID)
		if errors.Is(err, payables.ErrLinkNotFound) {
			writeProblem(w, 404, "payable-link-not-found", "Vínculo não encontrado", "")
			return
		}
		if err != nil {
			payableUnavailableProblem(w)
			return
		}
		if link.PayableID != *s.PayableID {
			invalidScenarioTransactionProblem(w, "O vínculo informado pertence a outra dívida ou conta a receber.")
			return
		}

		if _, err := scenarios.CreateRealization(r.Context(), conn, transactionID, &req.PayableLinkID, req.AllocatedAmount); err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		dto, err := scenarioTransactionDTOFor(r.Context(), conn, st)
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusCreated, dto)
	}
}

func handleDeleteRealization(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		transactionID, realizationID := r.PathValue("transactionId"), r.PathValue("realizationId")

		realization, err := scenarios.GetRealization(r.Context(), conn, realizationID)
		if errors.Is(err, scenarios.ErrRealizationNotFound) || (err == nil && realization.ScenarioTransactionID != transactionID) {
			realizationNotFoundProblem(w)
			return
		}
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		if err := scenarios.DeleteRealization(r.Context(), conn, realizationID); err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
