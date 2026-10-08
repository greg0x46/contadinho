package httpapi

import (
	"contadinho-go/internal/datasources"
	"contadinho-go/internal/settings"
	"database/sql"
	"net/http"
)

// handleQuotesSettings mirrors handlePluggySettings: write-only, and never
// echoes the saved value back. Unlike Pluggy's credentials, the brapi token
// is optional — the brapi provider in internal/marketdata sends requests
// unauthenticated when it is empty — so an empty body value is accepted too,
// as the deliberate way to clear a previously saved token.
func handleQuotesSettings(db *sql.DB, keys *settings.Secrets) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 16384)
		var req struct {
			BrapiToken string `json:"brapi_token"`
		}
		if decodeStrict(r, &req) != nil {
			writeProblem(w, 422, "invalid-settings", "Dados inválidos", "Não foi possível ler o token informado.")
			return
		}
		key, ok := keys.Key()
		if !ok {
			writeProblem(w, 503, "settings-unavailable", "Configuração indisponível", "")
			return
		}
		if err := settings.SetBrapiToken(r.Context(), db, req.BrapiToken, key); err != nil {
			writeProblem(w, 503, "settings-unavailable", "Não foi possível salvar o token", "")
			return
		}
		writeJSON(w, 200, map[string]bool{"saved": true})
	}
}

// Credentials are write-only and may be configured only by an authenticated owner.
func handlePluggySettings(db *sql.DB, keys *settings.Secrets) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 16384)
		var req struct {
			ClientID     string `json:"pluggy_client_id"`
			ClientSecret string `json:"pluggy_client_secret"`
			ItemID       string `json:"pluggy_item_id"`
		}
		if decodeStrict(r, &req) != nil || req.ClientID == "" || req.ClientSecret == "" {
			writeProblem(w, 422, "invalid-settings", "Credenciais inválidas", "Informe client ID e client secret da Pluggy.")
			return
		}
		key, ok := keys.Key()
		if !ok {
			writeProblem(w, 503, "settings-unavailable", "Configuração indisponível", "")
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err == nil {
			defer tx.Rollback()
			err = settings.Set(r.Context(), tx, "pluggy.client_id", req.ClientID, true, key)
			if err == nil {
				err = settings.Set(r.Context(), tx, "pluggy.client_secret", req.ClientSecret, true, key)
			}
			if err == nil && req.ItemID != "" {
				_, err = datasources.Create(r.Context(), tx, datasources.ProviderPluggy, req.ItemID, nil)
			}
			if err == nil {
				err = tx.Commit()
			}
		}
		if err != nil {
			writeProblem(w, 503, "settings-unavailable", "Não foi possível salvar as credenciais", "")
			return
		}
		writeJSON(w, 200, map[string]bool{"saved": true})
	}
}
