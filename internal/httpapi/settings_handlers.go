package httpapi

import (
	"contadinho-go/internal/datasources"
	"contadinho-go/internal/settings"
	"database/sql"
	"net/http"
)

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
