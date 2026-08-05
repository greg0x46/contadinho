package httpapi

import (
	"database/sql"
	"net/http"

	"contadinho-go/internal/settings"
)

type preferencesDTO struct {
	TransactionsPeriodBasis string `json:"transactions_period_basis"`
}

func handleGetPreferences(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		basis, err := settings.GetTransactionsPeriodBasis(r.Context(), db)
		if err != nil {
			writeProblem(w, 503, "preferences-unavailable", "Preferências temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		writeJSON(w, http.StatusOK, preferencesDTO{TransactionsPeriodBasis: basis})
	}
}

func handleUpdatePreferences(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req preferencesDTO
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-preferences", "Preferências inválidas", "Revise os campos enviados.")
			return
		}
		if req.TransactionsPeriodBasis != settings.PeriodBasisOccurredAt && req.TransactionsPeriodBasis != settings.PeriodBasisPaidAt {
			writeProblem(w, 422, "invalid-preferences", "Preferências inválidas",
				"transactions_period_basis deve ser \"occurred_at\" ou \"paid_at\".")
			return
		}
		if err := settings.Set(r.Context(), db, settings.KeyTransactionsPeriodBasis, req.TransactionsPeriodBasis, false, nil); err != nil {
			writeProblem(w, 503, "preferences-unavailable", "Preferências temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		writeJSON(w, http.StatusOK, req)
	}
}
