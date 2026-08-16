# M0 — Motor de regras compartilhado (`internal/rules`)

> Parte de `.specs/relatorio-financeiro.md`. Sem valor de UI direto — destrava
> M1 (reconciliação de recorrências) com risco zero de regressão em
> `internal/automation`.
>
> **Status: concluído.** A aliasing de `internal/automation/matching.go` para
> `internal/rules` (última etapa do escopo abaixo, originalmente adiada) foi
> feita junto com a reestruturação de `internal/recurrences` que reaproveita
> este motor por completo — ver `m1-recorrencias.md`, seção "Regra de
> conciliação configurável".

## Motivação

`internal/automation` já implementa um motor de correspondência combinável
(`Condition`/`Operator`/`LogicOperator`/`Matches`), usado hoje só para
auto-ignorar transações por texto (descrição/cartão/conta). M1 precisa do
mesmo tipo de motor para decidir, mês a mês, se uma transação real já
"satisfaz" a ocorrência esperada de um `RecurringCommitment` — mas por
categoria/valor/dia, não só texto. Em vez de duplicar a lógica de matching,
extraímos o núcleo para um pacote neutro e o estendemos.

## Escopo

- **Extrair**, sem mudar comportamento: `ConditionField`, `ConditionOperator`,
  `LogicOperator`, `Condition`, `MatchCandidate`, `normalize`, `textMatches`,
  `conditionMatches`, `Matches` de `internal/automation/matching.go` para
  `internal/rules/rules.go`.
- **Estender** `MatchCandidate` com `Amount *decimal.Decimal` e
  `DayOfMonth *int`.
- **Estender** `ConditionField` com `FieldAmount`, `FieldDayOfMonth`.
- **Estender** `ConditionOperator` com `OperatorWithinPercent` (tolerância
  percentual sobre `Amount`) e `OperatorNearDay` (tolerância em dias sobre
  `DayOfMonth`) — ambos parseiam `Condition.Value` como número na hora de
  avaliar (ex.: `"10"` = 10% ou 10 dias).
- `internal/automation/matching.go` vira um arquivo fino: tipos viram alias
  (`type Condition = rules.Condition`, etc.) e `Matches` delega para
  `rules.Matches`. Nenhum call-site em `internal/automation` ou
  `internal/httpapi/automation_handlers.go` muda.

## Contratos

```go
package rules

type ConditionField string
const (
    FieldDescription ConditionField = "description"
    FieldCard        ConditionField = "card"
    FieldAccount     ConditionField = "account"
    FieldAmount      ConditionField = "amount"
    FieldDayOfMonth  ConditionField = "day_of_month"
)

type ConditionOperator string
const (
    OperatorContains      ConditionOperator = "contains"
    OperatorEquals        ConditionOperator = "equals"
    OperatorWithinPercent ConditionOperator = "within_percent" // numeric tolerance on Amount
    OperatorNearDay       ConditionOperator = "near_day"       // day-of-month tolerance
)

type Condition struct {
    Field    ConditionField
    Operator ConditionOperator
    Value    string // text for contains/equals; a number for within_percent/near_day
}

type MatchCandidate struct {
    Description        *string
    CardNumber         *string
    AccountName        *string
    AccountInstitution *string
    Amount             *decimal.Decimal
    DayOfMonth         *int
}

func Matches(candidate MatchCandidate, conditions []Condition, logic LogicOperator) bool
```

`Matches` é agnóstico a "valor esperado" — só recebe o `MatchCandidate`
sendo testado, sem um segundo valor de referência externo — então a
referência precisa estar codificada no próprio `Condition.Value`. Convenção
de formato, igual para os dois operadores novos: `Value = "<referência>:<tolerância>"`.

- `within_percent`, `Value = "1500.00:10"` → casa se `candidate.Amount`
  estiver a até 10% de `1500.00` (`|amount - 1500| / 1500 <= 0.10`).
- `near_day`, `Value = "15:3"` → casa se `candidate.DayOfMonth` estiver a
  até 3 dias de `15` (considerando o wrap de fim de mês: distância mínima
  entre os dois valores).

Quem monta a `Condition` (M1, `internal/recurrences`) embute o valor/dia
esperado do compromisso mais a tolerância configurada — ver
`m1-recorrencias.md` para como essas condições são geradas a partir de um
`RecurringCommitment`.

## Critério de aceite

`go test ./internal/automation/... ./internal/rules/...` passa, e nenhuma
asserção de `internal/automation/matching_test.go` muda — a extração é
transparente para o call-site existente.
