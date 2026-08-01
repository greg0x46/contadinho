package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"contadinho-go/internal/categories"
	"contadinho-go/internal/money"
)

type categoryDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toCategoryDTO(c categories.Category) categoryDTO {
	return categoryDTO{
		ID:        c.ID,
		Name:      c.Name,
		Kind:      string(c.Kind),
		IsActive:  c.IsActive,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func handleListCategories(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := categories.List(r.Context(), db)
		if err != nil {
			writeProblem(w, 503, "categories-unavailable", "Categoria temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		dtos := make([]categoryDTO, len(list))
		for i, c := range list {
			dtos[i] = toCategoryDTO(c)
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

type categoryCreateRequest struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

var validCategoryKinds = map[string]bool{"expense": true, "income": true, "transfer": true}

func handleCreateCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req categoryCreateRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "Revise o nome e o tipo enviados.")
			return
		}
		if req.Name == "" || !validCategoryKinds[req.Kind] {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "Revise o nome e o tipo enviados.")
			return
		}
		c, err := categories.Create(r.Context(), db, req.Name, money.CategoryKind(req.Kind))
		if err != nil {
			writeProblem(w, 503, "category-unavailable", "Categoria temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		writeJSON(w, http.StatusCreated, toCategoryDTO(c))
	}
}

type categoryUpdateRequest struct {
	Name     *string `json:"name"`
	IsActive *bool   `json:"is_active"`
}

func handleUpdateCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req categoryUpdateRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "Revise o nome e o tipo enviados.")
			return
		}
		if req.Name == nil && req.IsActive == nil {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "Informe ao menos um campo para atualizar.")
			return
		}
		if req.Name != nil && *req.Name == "" {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "O nome não pode ser vazio.")
			return
		}
		c, err := categories.Update(r.Context(), db, id, req.Name, req.IsActive)
		if errors.Is(err, categories.ErrNotFound) {
			writeProblem(w, 404, "category-not-found", "Categoria não encontrada", "")
			return
		}
		if err != nil {
			writeProblem(w, 503, "category-unavailable", "Categoria temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		writeJSON(w, http.StatusOK, toCategoryDTO(c))
	}
}
