package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/debts"
	"contadinho-go/internal/money"
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
	DebtID    *string   `json:"debt_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func scenarioToDTO(s scenarios.Scenario) scenarioDTO {
	return scenarioDTO{
		ID: s.ID, Kind: string(s.Kind), Name: s.Name, DebtID: s.DebtID,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

type scenarioTransactionDTO struct {
	ID          string  `json:"id"`
	ScenarioID  string  `json:"scenario_id"`
	Description string  `json:"description"`
	Amount      string  `json:"amount"`
	ProjectedAt string  `json:"projected_at"`
	Category    *string `json:"category"`
}

func scenarioTransactionToDTO(st scenarios.ScenarioTransaction) scenarioTransactionDTO {
	return scenarioTransactionDTO{
		ID: st.ID, ScenarioID: st.ScenarioID, Description: st.Description,
		Amount:      money.CanonicalDecimal(st.Amount),
		ProjectedAt: st.ProjectedAt.Format(dateOnlyLayout),
		Category:    st.Category,
	}
}

type scenarioDetailDTO struct {
	scenarioDTO
	Transactions []scenarioTransactionDTO `json:"transactions"`
}

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
	dtos := make([]scenarioTransactionDTO, len(list))
	for i, st := range list {
		dtos[i] = scenarioTransactionToDTO(st)
	}
	return scenarioDetailDTO{scenarioDTO: scenarioToDTO(s), Transactions: dtos}, true
}

type scenarioCreateRequest struct {
	Name string `json:"name"`
}

// handleCreateDebtScenario creates a kind="debt_plan" Scenario for the debt
// named by the {id} path value — the only creation entry point in this v1,
// since "what_if" scenarios (no debt_id) have no UI or use case yet.
func handleCreateDebtScenario(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debtID := r.PathValue("id")
		if _, err := debts.Get(r.Context(), conn, debtID); errors.Is(err, debts.ErrNotFound) {
			debtNotFoundProblem(w)
			return
		} else if err != nil {
			debtUnavailableProblem(w)
			return
		}

		var req scenarioCreateRequest
		if err := decodeStrict(r, &req); err != nil || req.Name == "" {
			invalidScenarioProblem(w, "Informe um nome para o plano.")
			return
		}

		s, err := scenarios.CreateScenario(r.Context(), conn, scenarios.KindDebtPlan, req.Name, &debtID)
		if err != nil {
			scenariosUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusCreated, scenarioDetailDTO{scenarioDTO: scenarioToDTO(s), Transactions: []scenarioTransactionDTO{}})
	}
}

func handleListDebtScenarios(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debtID := r.PathValue("id")
		list, err := scenarios.ListScenariosByDebt(r.Context(), conn, debtID)
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
		writeJSON(w, http.StatusCreated, scenarioTransactionToDTO(st))
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
		writeJSON(w, http.StatusOK, scenarioTransactionToDTO(st))
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
