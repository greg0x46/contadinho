# M4 — Projeção base (payables + recorrências, sem cenários)

> Parte de `.specs/relatorio-financeiro.md`. Entrega sozinha: responde
> "quanto terei em dezembro", "qual meu menor saldo até lá", "vou ficar
> negativo" — usando só o que já é conhecido hoje (sem hipóteses de
> cenários, que chegam em M5).

## Motivação

M3 só olha para trás (Realizado). M4 estende o mesmo `internal/timeline`
para olhar para frente, usando exclusivamente fontes que já existem no
domínio hoje: parcelas futuras de planos de pagamento
(`Scenario{Kind: debt_plan|receivable_plan}`, já existentes) e ocorrências
de `RecurringCommitment` (M1). Nada além disso é inventado — não há
"recorrência automática detectada por heurística" nesta v1 (seção 3 do
doc-guarda-chuva).

## Extensão de `internal/timeline/build.go`

Dois novos passos entram no pipeline de `BuildSeries`, entre `realEntries` e
o merge final:

```go
// payablePlanEntries varre todo Scenario{Kind: debt_plan|receivable_plan}
// e suas ScenarioTransactions não realizadas (via
// ScenarioTransactionRealization) dentro de [From, To]. Sinal do Entry.Amount
// conforme Payable.Kind (debt = saída, receivable = entrada).
// Tier: Confirmado (é uma dívida/recebível real, com data prevista — não
// uma hipótese).
func payablePlanEntries(ctx context.Context, q Querier, from, to time.Time) ([]Entry, error)

// recurrenceEntries, para cada RecurringCommitment ativo (M1):
// recurrences.OccurrencesInRange(from, to) + recurrences.ResolveOccurrence
// contra as transações reais já carregadas em realEntries (mesmo conjunto,
// sem segunda query) — usa o valor real se casou (ainda Confirmado, mas
// com o valor de fato), o valor esperado se não casou.
func recurrenceEntries(ctx context.Context, q Querier, from, to time.Time, realCandidates []transactions.Item) ([]Entry, error)
```

Após merge+sort de todas as fontes por data:

```go
// runningBalance acumula a partir de StartingBalance, dia a dia.
// LowestBalance é o mínimo encontrado ao longo do caminho (com sua data) —
// seção 11 do pedido: "não apresentar apenas o saldo na data final".
// FirstNegative é a primeira data em que o saldo acumulado cruza para
// negativo (nil se nunca acontece) — seção 12.
```

`BuildParams.ReferenceDate` passa a ser usado de fato: separa o que é
"histórico" (`To <= ReferenceDate`, tier sempre Realizado/Confirmado já
ocorrido) do que é "projeção" (`Date > ReferenceDate`, tier
Confirmado/Projetado). O endpoint aceita uma data de referência arbitrária
no passado, presente ou futuro (seção 2/3 do pedido) — o comportamento muda
conforme:

- **Data passada**: prioriza Realizado (não há nada "confirmado futuro" a
  mostrar além do que já aconteceu até lá).
- **Data atual**: situação hoje = `StartingBalance` no ponto de partida.
- **Data futura**: projeção completa (Confirmado + Projetado) até lá.

## Endpoint HTTP

`GET /api/timeline` (M3) ganha `from`/`to`/`reference_date` efetivamente
plugados na projeção — não é uma rota nova, é a mesma rota do M3 com o
pipeline de `BuildSeries` agora populando fontes futuras.

## Frontend

- `frontend/src/components/timeline/ProjectionTimeline.tsx` — Recharts,
  linha única unindo Realizado→Confirmado→Projetado num eixo de tempo
  contínuo (seção 10 do pedido). Marcador vertical em "hoje". Marcador no
  ponto de `Series.LowestBalance`. Se `Series.FirstNegative != nil`, uma
  anotação neutra (não alarmista, seção 12) indicando a data.
- `frontend/src/components/timeline/EntryOriginBadge.tsx` — badge pequeno
  distinguindo Realizado/Confirmado/Recorrente na composição da projeção
  (antecipa a seção 26/27 completa, que só fecha no M5 com Hipotético
  incluído).
- Cards de saldo: "Saldo atual" (hoje) e "Saldo projetado" (na data
  selecionada), com "Menor saldo até lá" + sua data — seções 9/11.

## Validação específica deste milestone

Fixture com dois casos desenhados de propósito (detalhado na seção
"Validação" do doc-guarda-chuva):

1. Vale de saldo conhecido: valores arranjados para que o saldo mínimo
   ocorra numa data específica antes da data de referência final — assert
   exato de `Series.LowestBalance.Date` e `.Balance`.
2. Saldo negativo: variante com saídas superando entradas de propósito,
   para testar `FirstNegative != nil` e a data exata em que cruza.

## Critério de aceite

- `go test ./internal/timeline/...`: `payablePlanEntries`,
  `recurrenceEntries` (com e sem match — usa `internal/recurrences` do M1
  diretamente), `LowestBalance`/`FirstNegative` cobertos com os dois casos
  de fixture acima.
- Navegação manual: selecionar uma data futura no `/relatorio-financeiro` e
  conferir saldo projetado, menor saldo no caminho e (se aplicável) alerta
  de saldo negativo — sem nenhum cenário ativo ainda.
