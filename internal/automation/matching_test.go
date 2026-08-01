package automation_test

import (
	"testing"

	"contadinho-go/internal/automation"
)

func strp(s string) *string { return &s }

func TestMatchesDescriptionContainsIsCaseInsensitive(t *testing.T) {
	candidate := automation.MatchCandidate{Description: strp("PIX Enviado Mercado Livre")}
	conditions := []automation.Condition{{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "mercado livre"}}
	if !automation.Matches(candidate, conditions, automation.LogicAnd) {
		t.Error("expected a case-insensitive substring match")
	}
}

func TestMatchesDescriptionEqualsRequiresExactNormalizedMatch(t *testing.T) {
	candidate := automation.MatchCandidate{Description: strp("  Uber Trip  ")}
	conditions := []automation.Condition{{Field: automation.FieldDescription, Operator: automation.OperatorEquals, Value: "uber trip"}}
	if !automation.Matches(candidate, conditions, automation.LogicAnd) {
		t.Error("expected equals to match after trim+casefold normalization")
	}

	conditions = []automation.Condition{{Field: automation.FieldDescription, Operator: automation.OperatorEquals, Value: "uber"}}
	if automation.Matches(candidate, conditions, automation.LogicAnd) {
		t.Error("equals should not match a mere substring")
	}
}

func TestMatchesNilCandidateFieldNeverMatches(t *testing.T) {
	candidate := automation.MatchCandidate{}
	conditions := []automation.Condition{{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "anything"}}
	if automation.Matches(candidate, conditions, automation.LogicAnd) {
		t.Error("a nil candidate field should never match")
	}
}

func TestMatchesCardField(t *testing.T) {
	candidate := automation.MatchCandidate{CardNumber: strp("**** 1234")}
	conditions := []automation.Condition{{Field: automation.FieldCard, Operator: automation.OperatorContains, Value: "1234"}}
	if !automation.Matches(candidate, conditions, automation.LogicAnd) {
		t.Error("expected card field match")
	}
}

func TestMatchesAccountFieldChecksNameOrInstitution(t *testing.T) {
	byName := automation.MatchCandidate{AccountName: strp("Conta Corrente"), AccountInstitution: strp("Banco X")}
	byInstitution := automation.MatchCandidate{AccountName: strp("Poupança"), AccountInstitution: strp("Nubank")}
	conditions := []automation.Condition{{Field: automation.FieldAccount, Operator: automation.OperatorContains, Value: "nubank"}}

	if automation.Matches(byName, conditions, automation.LogicAnd) {
		t.Error("should not match when neither name nor institution contains the value")
	}
	if !automation.Matches(byInstitution, conditions, automation.LogicAnd) {
		t.Error("should match via institution when name doesn't match")
	}
}

func TestMatchesLogicAndRequiresAllConditions(t *testing.T) {
	candidate := automation.MatchCandidate{Description: strp("Uber Trip"), CardNumber: strp("1234")}
	conditions := []automation.Condition{
		{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "uber"},
		{Field: automation.FieldCard, Operator: automation.OperatorEquals, Value: "9999"},
	}
	if automation.Matches(candidate, conditions, automation.LogicAnd) {
		t.Error("AND should fail when one condition doesn't match")
	}
	if !automation.Matches(candidate, conditions, automation.LogicOr) {
		t.Error("OR should succeed when at least one condition matches")
	}
}
