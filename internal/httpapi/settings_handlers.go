package httpapi

import (
	"database/sql"
	"errors"
	"net/http"

	"contadinho-go/internal/settings"
)

type setupStatusDTO struct {
	Configured bool `json:"configured"`
	Unlocked   bool `json:"unlocked"`
}

func handleSetupStatus(db *sql.DB, session *settings.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configured, err := settings.IsConfigured(r.Context(), db)
		if err != nil {
			writeProblem(w, 503, "setup-unavailable", "Configuração temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		_, unlocked := session.Key()
		writeJSON(w, http.StatusOK, setupStatusDTO{Configured: configured, Unlocked: unlocked})
	}
}

type setupRequest struct {
	Password           string `json:"password"`
	PluggyClientID     string `json:"pluggy_client_id"`
	PluggyClientSecret string `json:"pluggy_client_secret"`
	PluggyItemID       string `json:"pluggy_item_id"`
}

// handleSetup runs the one-time setup screen: sets the passphrase that
// derives the at-rest encryption key, and stores the initial Pluggy
// credentials (client_id/client_secret encrypted, item_id plain — it
// identifies a connection rather than authenticating one). It unlocks the
// session immediately so sync can start without asking the user to re-enter
// the password they just chose.
func handleSetup(db *sql.DB, session *settings.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req setupRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-setup", "Configuração inválida", "Revise os campos enviados.")
			return
		}
		if len(req.Password) < 8 || req.PluggyClientID == "" || req.PluggyClientSecret == "" || req.PluggyItemID == "" {
			writeProblem(w, 422, "invalid-setup", "Configuração inválida",
				"A senha deve ter ao menos 8 caracteres e as credenciais do Pluggy são obrigatórias.")
			return
		}

		key, err := settings.Setup(r.Context(), db, req.Password)
		if errors.Is(err, settings.ErrAlreadyConfigured) {
			writeProblem(w, 409, "already-configured", "Aplicação já configurada", "")
			return
		}
		if err != nil {
			writeProblem(w, 503, "setup-unavailable", "Configuração temporariamente indisponível", "Tente novamente em instantes.")
			return
		}

		if err := settings.Set(r.Context(), db, "pluggy.client_id", req.PluggyClientID, true, key); err != nil {
			writeProblem(w, 503, "setup-unavailable", "Configuração temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		if err := settings.Set(r.Context(), db, "pluggy.client_secret", req.PluggyClientSecret, true, key); err != nil {
			writeProblem(w, 503, "setup-unavailable", "Configuração temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		if err := settings.Set(r.Context(), db, "pluggy.item_id", req.PluggyItemID, false, nil); err != nil {
			writeProblem(w, 503, "setup-unavailable", "Configuração temporariamente indisponível", "Tente novamente em instantes.")
			return
		}

		session.Unlock(key)
		writeJSON(w, http.StatusCreated, setupStatusDTO{Configured: true, Unlocked: true})
	}
}

type unlockRequest struct {
	Password string `json:"password"`
}

func handleUnlock(db *sql.DB, session *settings.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req unlockRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-unlock", "Solicitação de desbloqueio inválida", "Informe a senha.")
			return
		}
		key, err := settings.VerifyPassword(r.Context(), db, req.Password)
		if errors.Is(err, settings.ErrNotConfigured) {
			writeProblem(w, 409, "not-configured", "Aplicação ainda não configurada", "")
			return
		}
		if errors.Is(err, settings.ErrIncorrectPassword) {
			writeProblem(w, 401, "incorrect-password", "Senha incorreta", "")
			return
		}
		if err != nil {
			writeProblem(w, 503, "unlock-unavailable", "Desbloqueio temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		session.Unlock(key)
		writeJSON(w, http.StatusOK, setupStatusDTO{Configured: true, Unlocked: true})
	}
}
