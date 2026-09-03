package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"contadinho-go/internal/datasources"
)

// connectionNameColumn is DataSource.Name expressed in SQL, for the queries
// that join a connection in rather than loading one through the datasources
// package: the user's label wins, then the institution the last sync
// reported, then the raw item id — which is all a connection added but never
// synced has. Every caller aliases data_sources as ds.
//
// It exists once because three separate spellings of the same fallback chain
// is three chances to disagree: a never-synced connection would then be named
// by its item id in the sync history and by nothing at all in the account list.
const connectionNameColumn = `COALESCE(NULLIF(ds.label, ''), NULLIF(ds.display_name, ''), ds.external_item_id)`

type dataSourceDTO struct {
	ID             string    `json:"id"`
	Provider       string    `json:"provider"`
	ExternalItemID string    `json:"external_item_id"`
	DisplayName    *string   `json:"display_name"`
	Label          *string   `json:"label"`
	Name           string    `json:"name"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toDataSourceDTO(d datasources.DataSource) dataSourceDTO {
	return dataSourceDTO{
		ID:             d.ID,
		Provider:       d.Provider,
		ExternalItemID: d.ExternalItemID,
		DisplayName:    d.DisplayName,
		Label:          d.Label,
		Name:           d.Name(),
		IsActive:       d.IsActive,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

// handleGetDataSource backs the Location header handleCreateDataSource sets,
// and gives a client a way to re-read one connection without listing them all.
func handleGetDataSource(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source, err := datasources.Get(r.Context(), conn, r.PathValue("id"))
		if errors.Is(err, datasources.ErrNotFound) {
			writeProblem(w, 404, "data-source-not-found", "Conexão não encontrada", "")
			return
		}
		if err != nil {
			writeProblem(w, 503, "data-sources-unavailable", "Conexões temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		writeJSON(w, http.StatusOK, toDataSourceDTO(source))
	}
}

func handleListDataSources(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sources, err := datasources.List(r.Context(), conn)
		if err != nil {
			writeProblem(w, 503, "data-sources-unavailable", "Conexões temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		result := make([]dataSourceDTO, 0, len(sources))
		for _, source := range sources {
			result = append(result, toDataSourceDTO(source))
		}
		writeJSON(w, http.StatusOK, result)
	}
}

type createDataSourceRequest struct {
	ExternalItemID string  `json:"external_item_id"`
	Label          *string `json:"label"`
}

func handleCreateDataSource(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createDataSourceRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-data-source", "Conexão inválida", "Revise os campos enviados.")
			return
		}
		itemID := strings.TrimSpace(req.ExternalItemID)
		if itemID == "" {
			writeProblem(w, 422, "invalid-data-source", "Conexão inválida", "Informe o Item ID da conexão no Pluggy.")
			return
		}

		source, err := datasources.Create(r.Context(), conn, datasources.ProviderPluggy, itemID, trimmedLabel(req.Label))
		if errors.Is(err, datasources.ErrDuplicate) {
			writeProblem(w, 409, "data-source-exists", "Conexão já cadastrada",
				"Esse Item ID já está sincronizando neste Contadinho.")
			return
		}
		if err != nil {
			writeProblem(w, 503, "data-sources-unavailable", "Conexões temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		w.Header().Set("Location", "/api/data-sources/"+source.ID)
		writeJSON(w, http.StatusCreated, toDataSourceDTO(source))
	}
}

type updateDataSourceRequest struct {
	Label    *string `json:"label"`
	IsActive *bool   `json:"is_active"`
}

func handleUpdateDataSource(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req updateDataSourceRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-data-source", "Conexão inválida", "Revise os campos enviados.")
			return
		}
		if req.Label == nil && req.IsActive == nil {
			writeProblem(w, 422, "invalid-data-source", "Conexão inválida", "Nada a alterar.")
			return
		}

		source, err := datasources.Update(r.Context(), conn, r.PathValue("id"), trimmedLabel(req.Label), req.IsActive)
		if errors.Is(err, datasources.ErrNotFound) {
			writeProblem(w, 404, "data-source-not-found", "Conexão não encontrada", "")
			return
		}
		if err != nil {
			writeProblem(w, 503, "data-sources-unavailable", "Conexões temporariamente indisponíveis", "Tente novamente em instantes.")
			return
		}
		writeJSON(w, http.StatusOK, toDataSourceDTO(source))
	}
}

// trimmedLabel preserves the nil/non-nil distinction Update relies on to tell
// "leave the label alone" from "clear it" — only the string inside is
// trimmed, so sending "  " clears the label rather than storing whitespace.
func trimmedLabel(label *string) *string {
	if label == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*label)
	return &trimmed
}
