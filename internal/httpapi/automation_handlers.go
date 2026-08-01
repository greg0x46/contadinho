package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"contadinho-go/internal/automation"
)

type conditionDTO struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type ruleDTO struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	IsActive      bool           `json:"is_active"`
	LogicOperator string         `json:"logic_operator"`
	Conditions    []conditionDTO `json:"conditions"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func toRuleDTO(r automation.Rule) ruleDTO {
	conditions := make([]conditionDTO, len(r.Conditions))
	for i, c := range r.Conditions {
		conditions[i] = conditionDTO{Field: string(c.Field), Operator: string(c.Operator), Value: c.Value}
	}
	return ruleDTO{
		ID: r.ID, Name: r.Name, IsActive: r.IsActive, LogicOperator: string(r.LogicOperator),
		Conditions: conditions, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

var (
	validFields    = map[string]bool{"description": true, "card": true, "account": true}
	validOperators = map[string]bool{"contains": true, "equals": true}
	validLogic     = map[string]bool{"and": true, "or": true}
)

type ruleWriteRequest struct {
	Name               string         `json:"name"`
	IsActive           bool           `json:"is_active"`
	LogicOperator      string         `json:"logic_operator"`
	Conditions         []conditionDTO `json:"conditions"`
	ApplyRetroactively bool           `json:"apply_retroactively"`
}

func (req ruleWriteRequest) toWrite() (automation.Write, bool) {
	if req.Name == "" || len(req.Conditions) == 0 || !validLogic[req.LogicOperator] {
		return automation.Write{}, false
	}
	conditions := make([]automation.Condition, len(req.Conditions))
	for i, c := range req.Conditions {
		if c.Value == "" || !validFields[c.Field] || !validOperators[c.Operator] {
			return automation.Write{}, false
		}
		conditions[i] = automation.Condition{Field: automation.ConditionField(c.Field), Operator: automation.ConditionOperator(c.Operator), Value: c.Value}
	}
	return automation.Write{
		Name: req.Name, IsActive: req.IsActive, LogicOperator: automation.LogicOperator(req.LogicOperator), Conditions: conditions,
	}, true
}

type retroactiveResultDTO struct {
	Matched int `json:"matched"`
	Ignored int `json:"ignored"`
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
		if err != nil {
			automationRuleUnavailableProblem(w)
			return
		}
		result := ruleWriteResultDTO{Rule: toRuleDTO(rule)}
		if req.ApplyRetroactively {
			outcome, err := automation.ApplyRetroactively(r.Context(), conn, rule.ID, onIgnoredHook)
			if err != nil {
				automationRuleUnavailableProblem(w)
				return
			}
			result.RetroactiveApply = &retroactiveResultDTO{Matched: outcome.Matched, Ignored: outcome.Ignored}
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
		if err != nil {
			automationRuleUnavailableProblem(w)
			return
		}
		result := ruleWriteResultDTO{Rule: toRuleDTO(rule)}
		if req.ApplyRetroactively {
			outcome, err := automation.ApplyRetroactively(r.Context(), conn, rule.ID, onIgnoredHook)
			if err != nil {
				automationRuleUnavailableProblem(w)
				return
			}
			result.RetroactiveApply = &retroactiveResultDTO{Matched: outcome.Matched, Ignored: outcome.Ignored}
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
