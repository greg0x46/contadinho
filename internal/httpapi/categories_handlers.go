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
	Icon      string    `json:"icon"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toCategoryDTO(c categories.Category) categoryDTO {
	return categoryDTO{
		ID:        c.ID,
		Name:      c.Name,
		Kind:      string(c.Kind),
		IsActive:  c.IsActive,
		Icon:      c.Icon,
		Color:     c.Color,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// validCategoryIcons is the curated icon-key set a category's icon must
// belong to. Kept in sync by hand with categoryIconRegistry in
// frontend/src/presentation/categoryLabels.ts — there is no shared schema
// between the two, so any addition here needs a matching addition there.
var validCategoryIcons = map[string]bool{
	"shopping-cart": true, "coffee": true, "car": true, "shopping": true,
	"bank": true, "home": true, "smile": true, "percentage": true,
	"medicine": true, "scissor": true, "team": true, "tool": true,
	"wifi": true, "book": true, "safety-certificate": true, "heart": true,
	"global": true, "gift": true, "line-chart": true, "ellipsis": true,
	"money-collect": true, "laptop": true, "trophy": true, "rise": true,
	"rollback": true, "wallet": true, "plus-circle": true, "swap": true,
	"phone": true, "fund": true, "dollar": true, "cloud": true,
	"hourglass": true, "thunderbolt": true, "credit-card": true,
	"customer-service": true, "fire": true, "bulb": true,
}

// validCategoryColors is the curated hex palette a category's color must
// belong to. Kept in sync by hand with categoryColorPalette in
// frontend/src/presentation/categoryLabels.ts.
var validCategoryColors = map[string]bool{
	"#2a78d6": true, "#eb6834": true, "#17a2b8": true, "#e64980": true,
	"#d64545": true, "#b8860b": true, "#eda100": true, "#495057": true,
	"#37b24d": true, "#e87ba4": true, "#4c6ef5": true, "#6c757d": true,
	"#099268": true, "#7c5cbf": true, "#1baf7a": true, "#f76707": true,
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
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

var validCategoryKinds = map[string]bool{"expense": true, "income": true, "transfer": true}

func handleCreateCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req categoryCreateRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "Revise o nome, o tipo, o ícone e a cor enviados.")
			return
		}
		if req.Name == "" || !validCategoryKinds[req.Kind] || !validCategoryIcons[req.Icon] || !validCategoryColors[req.Color] {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "Revise o nome, o tipo, o ícone e a cor enviados.")
			return
		}
		c, err := categories.Create(r.Context(), db, req.Name, money.CategoryKind(req.Kind), req.Icon, req.Color)
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
	Icon     *string `json:"icon"`
	Color    *string `json:"color"`
}

func handleUpdateCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req categoryUpdateRequest
		if err := decodeStrict(r, &req); err != nil {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "Revise o nome, o ícone e a cor enviados.")
			return
		}
		if req.Name == nil && req.IsActive == nil && req.Icon == nil && req.Color == nil {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "Informe ao menos um campo para atualizar.")
			return
		}
		if req.Name != nil && *req.Name == "" {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "O nome não pode ser vazio.")
			return
		}
		if req.Icon != nil && !validCategoryIcons[*req.Icon] {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "O ícone informado não é válido.")
			return
		}
		if req.Color != nil && !validCategoryColors[*req.Color] {
			writeProblem(w, 422, "invalid-category", "Categoria inválida", "A cor informada não é válida.")
			return
		}
		c, err := categories.Update(r.Context(), db, id, req.Name, req.IsActive, req.Icon, req.Color)
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
