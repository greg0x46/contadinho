package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"time"

	"contadinho-go/internal/automation"
)

type actionDTO struct {
	Type                  string  `json:"type"`
	RecurringCommitmentID *string `json:"recurring_commitment_id"`
	CategoryID            *string `json:"category_id"`
}

func toActionDTOs(actions []automation.Action) []actionDTO {
	dtos := make([]actionDTO, len(actions))
	for i, a := range actions {
		dtos[i] = actionDTO{Type: string(a.Type), RecurringCommitmentID: a.RecurringCommitmentID, CategoryID: a.CategoryID}
	}
	return dtos
}

func actionsFromDTOs(dtos []actionDTO) []automation.ActionWrite {
	actions := make([]automation.ActionWrite, len(dtos))
	for i, d := range dtos {
		actions[i] = automation.ActionWrite{
			Type: automation.ActionType(d.Type), RecurringCommitmentID: d.RecurringCommitmentID, CategoryID: d.CategoryID,
		}
	}
	return actions
}

func hasActionType(actions []automation.ActionWrite, t automation.ActionType) bool {
	return slices.ContainsFunc(actions, func(a automation.ActionWrite) bool { return a.Type == t })
}

type ruleDTO struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	IsActive      bool           `json:"is_active"`
	LogicOperator string         `json:"logic_operator"`
	Conditions    []conditionDTO `json:"conditions"`
	Actions       []actionDTO    `json:"actions"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func toRuleDTO(r automation.Rule) ruleDTO {
	return ruleDTO{
		ID: r.ID, Name: r.Name, IsActive: r.IsActive, LogicOperator: string(r.LogicOperator),
		Conditions: toConditionDTOs(r.Conditions), Actions: toActionDTOs(r.Actions),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

var (
	automationFieldOperators = map[string]map[string]bool{
		"description": {"contains": true, "equals": true},
		"card":        {"contains": true, "equals": true},
		"account":     {"contains": true, "equals": true},
	}
	reconcileOnlyFieldOperators = map[string]map[string]bool{
		"amount":       {"within_percent": true},
		"day_of_month": {"day_range": true},
	}
)

// allowedAutomationCondition returns the condition allowlist for a write:
// description/card/account are always allowed, amount/day_of_month only
// when the write has a reconcile action (see automation.Write.Validate's
// doc comment for why).
func allowedAutomationCondition(hasReconcileAction bool) func(field, operator string) bool {
	return func(field, operator string) bool {
		if automationFieldOperators[field][operator] {
			return true
		}
		return hasReconcileAction && reconcileOnlyFieldOperators[field][operator]
	}
}

type ruleWriteRequest struct {
	Name               string         `json:"name"`
	IsActive           bool           `json:"is_active"`
	LogicOperator      string         `json:"logic_operator"`
	Conditions         []conditionDTO `json:"conditions"`
	Actions            []actionDTO    `json:"actions"`
	ApplyRetroactively bool           `json:"apply_retroactively"`
}

func (req ruleWriteRequest) toWrite() (automation.Write, bool) {
	actions := actionsFromDTOs(req.Actions)
	hasReconcile := hasActionType(actions, automation.ActionReconcile)
	conditions, ok := conditionsFromDTOs(req.Conditions, allowedAutomationCondition(hasReconcile))
	if !ok {
		return automation.Write{}, false
	}
	write := automation.Write{
		Name: req.Name, IsActive: req.IsActive, LogicOperator: automation.LogicOperator(req.LogicOperator),
		Conditions: conditions, Actions: actions,
	}
	if err := write.Validate(); err != nil {
		return automation.Write{}, false
	}
	return write, true
}

type retroactiveResultDTO struct {
	Matched     int `json:"matched"`
	Ignored     int `json:"ignored"`
	Categorized int `json:"categorized"`
}

type ruleWriteResultDTO struct {
	Rule             ruleDTO               `json:"rule"`
	RetroactiveApply *retroactiveResultDTO `json:"retroactive_apply"`
}

func invalidAutomationRuleProblem(w http.ResponseWriter) {
	writeProblem(w, 422, "invalid-automation-rule", "Regra de automação inválida", "Revise o nome, o operador lógico e as condições enviadas.")
}

func automationRuleUnavailableProblem(w http.ResponseWriter) {
	writeProblem(w, 503, "automation-rule-unavailable", "Automação temporariamente indisponível", "Tente novamente em instantes.")
}

func handleListConditionOptions(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accounts, cards, err := automation.ListConditionOptions(r.Context(), conn)
		if err != nil {
			writeProblem(w, 503, "automation-rule-options-unavailable", "Automação temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		if accounts == nil {
			accounts = []string{}
		}
		if cards == nil {
			cards = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts, "cards": cards})
	}
}

func handleListAutomationRules(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rules, err := automation.List(r.Context(), conn)
		if err != nil {
			automationRuleUnavailableProblem(w)
			return
		}
		dtos := make([]ruleDTO, len(rules))
		for i, rule := range rules {
			dtos[i] = toRuleDTO(rule)
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

func handleCreateAutomationRule(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ruleWriteRequest
		if err := decodeStrict(r, &req); err != nil {
			invalidAutomationRuleProblem(w)
			return
		}
		write, ok := req.toWrite()
		if !ok {
			invalidAutomationRuleProblem(w)
			return
		}
		rule, err := automation.Create(r.Context(), conn, write)
		if errors.Is(err, automation.ErrInvalidActionTarget) {
			invalidAutomationRuleProblem(w)
			return
		}
		if err != nil {
			automationRuleUnavailableProblem(w)
			return
		}
		result := ruleWriteResultDTO{Rule: toRuleDTO(rule)}
		if req.ApplyRetroactively && (hasActionType(write.Actions, automation.ActionIgnore) || hasActionType(write.Actions, automation.ActionSetCategory)) {
			outcome, err := automation.ApplyRetroactively(r.Context(), conn, rule.ID, onIgnoredHook)
			if err != nil {
				automationRuleUnavailableProblem(w)
				return
			}
			result.RetroactiveApply = &retroactiveResultDTO{Matched: outcome.Matched, Ignored: outcome.Ignored, Categorized: outcome.Categorized}
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

func handleUpdateAutomationRule(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req ruleWriteRequest
		if err := decodeStrict(r, &req); err != nil {
			invalidAutomationRuleProblem(w)
			return
		}
		write, ok := req.toWrite()
		if !ok {
			invalidAutomationRuleProblem(w)
			return
		}
		rule, err := automation.Update(r.Context(), conn, id, write)
		if errors.Is(err, automation.ErrNotFound) {
			writeProblem(w, 404, "automation-rule-not-found", "Regra não encontrada", "")
			return
		}
		if errors.Is(err, automation.ErrInvalidActionTarget) {
			invalidAutomationRuleProblem(w)
			return
		}
		if err != nil {
			automationRuleUnavailableProblem(w)
			return
		}
		result := ruleWriteResultDTO{Rule: toRuleDTO(rule)}
		if req.ApplyRetroactively && (hasActionType(write.Actions, automation.ActionIgnore) || hasActionType(write.Actions, automation.ActionSetCategory)) {
			outcome, err := automation.ApplyRetroactively(r.Context(), conn, rule.ID, onIgnoredHook)
			if err != nil {
				automationRuleUnavailableProblem(w)
				return
			}
			result.RetroactiveApply = &retroactiveResultDTO{Matched: outcome.Matched, Ignored: outcome.Ignored, Categorized: outcome.Categorized}
		}
		writeJSON(w, http.StatusOK, result)
	}
}

type activationRequest struct {
	IsActive bool `json:"is_active"`
}

func handleSetAutomationRuleActive(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req activationRequest
		if err := decodeStrict(r, &req); err != nil {
			invalidAutomationRuleProblem(w)
			return
		}
		rule, err := automation.SetActive(r.Context(), conn, id, req.IsActive)
		if errors.Is(err, automation.ErrNotFound) {
			writeProblem(w, 404, "automation-rule-not-found", "Regra não encontrada", "")
			return
		}
		if err != nil {
			automationRuleUnavailableProblem(w)
			return
		}
		writeJSON(w, http.StatusOK, toRuleDTO(rule))
	}
}

func handleDeleteAutomationRule(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := automation.Delete(r.Context(), conn, id)
		if errors.Is(err, automation.ErrNotFound) {
			writeProblem(w, 404, "automation-rule-not-found", "Regra não encontrada", "")
			return
		}
		if err != nil {
			automationRuleUnavailableProblem(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
