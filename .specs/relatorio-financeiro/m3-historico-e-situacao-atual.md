# M3 — Relatório Financeiro: histórico e situação atual (`internal/timeline`, sem projeção)

> Parte de `.specs/relatorio-financeiro.md`. Entrega sozinha: responde
> "quanto gastei em agosto", "quanto recebi no ano", "onde estou gastando
> mais" — sem depender de M4 (projeção) nem M5 (cenários).

## Motivação

É a fundação do relatório: um motor central (`internal/timeline`) que vira a
**única** fonte consumida por cards, gráfico e drill-down — nunca cada
camada de apresentação recalculando por conta própria (reconciliação de
totais, seções 29/36 do pedido original). Nesta primeira fatia, a série só
contém o que já é **Realizado** (transações reais via
`internal/transactions`); M4 adiciona Confirmado/Projetado, M5 adiciona
Hipotético.

## Modelo de dados

```go
package timeline

type CertaintyTier string
const (
    TierRealizado  CertaintyTier = "realizado"
    TierConfirmado CertaintyTier = "confirmado"
    TierProjetado  CertaintyTier = "projetado"
    TierHipotetico CertaintyTier = "hipotetico"
)

type SourceKind string
const (
    SourceReal        SourceKind = "real"
    SourceRecurring   SourceKind = "recorrente"
    SourcePayablePlan SourceKind = "plano_pagamento"
    SourceScenario    SourceKind = "cenario"
)

// Entry is one atomic cash-flow item — the same shape consumed by cards,
// chart, and drill-down (never a second, parallel representation).
type Entry struct {
    Date         time.Time
    Description  string
    Amount       decimal.Decimal // signed: positive=inflow, negative=outflow
    CategoryID   *string         // nil means "Sem categoria" — never dropped from totals
    CategoryName string
    Tier         CertaintyTier
    Source       SourceKind
    SourceRefID  string
    ScenarioID   *string // set only when Source == SourceScenario (from M5 on)
}

type DayPoint struct {
    Date       time.Time
    Balance    decimal.Decimal
    Inflow     decimal.Decimal
    Outflow    decimal.Decimal
    LowestTier CertaintyTier
}

type Series struct {
    Points          []DayPoint
    Entries         []Entry
    StartingBalance decimal.Decimal
    LowestBalance   DayPoint    // preenchido de fato a partir de M4 (sem fontes futuras, M3 é sempre "flat" no presente)
    FirstNegative   *time.Time  // idem — nil nesta fatia salvo transações reais já negativas
}
```

Nesta fatia (M3), `BuildParams` e `BuildSeries` só produzem entradas
`SourceReal`/`TierRealizado`:

```go
type BuildParams struct {
    From, To      time.Time
    ReferenceDate time.Time
    AccountIDs, CategoryIDs, CardNumbers []string
}

func BuildSeries(ctx context.Context, db Querier, params BuildParams) (Series, error)
```

Passos internos (isoláveis, cada um testável):

1. `startingBalance` — soma `financial_accounts.balance` filtrado por
   `AccountIDs`.
2. `realEntries` — via `transactions.Query` (filtros já suportados:
   `DateFrom/DateTo`, `AccountID`, `CategoryID`), mapeando `transactions.Item`
   → `Entry{Tier: Realizado, Source: real}`.
3. merge+sort por data → saldo corrente acumulado a partir de
   `StartingBalance` (útil já aqui para "saldo até hoje", mesmo sem fontes
   futuras).

`internal/timeline/aggregate.go`:

```go
// MonthlyBreakdown: uma linha por mês no intervalo, para o gráfico de
// evolução mensal (seção 6 do pedido).
func MonthlyBreakdown(series Series) []MonthSummary

type MonthSummary struct {
    Month    time.Time // primeiro dia do mês
    Income   decimal.Decimal
    Expense  decimal.Decimal
    Result   decimal.Decimal // Income - Expense
}

// CategoryBreakdown: despesas do mês agrupadas por categoria, ordenado por
// impacto absoluto decrescente. "Sem categoria" (CategoryID == nil) sempre
// presente na lista, mesmo com valor zero — nunca ignorada nos totais.
func CategoryBreakdown(series Series, month time.Time) []CategoryImpact

type CategoryImpact struct {
    CategoryID   *string
    CategoryName string // "Sem categoria" quando CategoryID == nil
    Amount       decimal.Decimal
    Percentage   decimal.Decimal // Amount / total das despesas do mês
}
```

## Endpoint HTTP

`internal/httpapi/timeline_handlers.go`:

```
GET /api/timeline?reference_date=2026-08-15&from=2026-01-01&to=2026-08-31&account_ids=...&category_ids=...&card_numbers=...
```

Nesta fatia, sem `scenario_ids` — só retorna `{base: Series}` (sem
`simulation`/`scenario_impacts`, que chegam em M5). DTOs espelham
`Series`/`Entry`/`MonthSummary`/`CategoryImpact`, `Amount` sempre string BRL
(`money.CanonicalDecimal`), seguindo o padrão de
`internal/httpapi/transactions_handlers.go`.

## Frontend

- `frontend/src/api/timeline.ts` — `getTimeline(params, signal)`.
- `frontend/src/api/contracts.ts` — `CertaintyTier`, `TimelineEntry`,
  `TimelineDayPoint`, `TimelineSeries`, `MonthSummary`, `CategoryImpact`,
  `TimelineResponse`, `parseTimelineResponse`.
- `frontend/src/hooks/useTimeline.ts` — `useQuery` com
  `queryKey: ["timeline", params]`.
- Introduzir **Recharts** (`package.json`) — primeira vez que uma lib de
  gráfico entra no frontend (hoje dashboards são CSS puro/meters via antd).
- `frontend/src/pages/FinancialReportPage.tsx` — dentro de `PageContainer`
  (padrão existente), usando `LoadingState`/`UnavailableState` de
  `components/AsyncState.tsx`.
- `frontend/src/components/timeline/`:
  - `TimeNavigator.tsx` — seletor ano (`< 2025 | 2026 | 2027 >`) + mês
    (`Jan..Dez`) + campo de data de referência específica.
  - `SummaryCards.tsx` — receitas/despesas/resultado/saldo do mês
    selecionado **e** acumulado no ano, visualmente distintos (seção 4/5 do
    pedido — nunca misturar as duas perspectivas no mesmo número).
  - `MonthlyEvolutionChart.tsx` — Recharts, uma linha/barra por mês
    (receitas/despesas/resultado).
  - `AccumulatedResultCard.tsx` — evolução do resultado acumulado mês a mês
    no ano (seção 7).
  - `CategoryImpactList.tsx` — lista ordenada por impacto; clicar navega
    para `/transacoes?category_id=X&date_from=Y&date_to=Z` (reaproveita
    `ConfigurableFilters`/`filterUrl.ts` — **não** cria segunda listagem de
    transações).
- Rota `/relatorio-financeiro` em `app/router.tsx` + item de menu em
  `app/App.tsx`.

## Estados a tratar (antecipando seção 31 do pedido, já nesta fatia)

- Carregando (`LoadingState`).
- Ano/mês sem nenhuma movimentação: distinguir explicitamente de "R$ 0,00"
  — mostrar mensagem "sem movimentações no período", não um card zerado
  ambíguo.
- Erro de carregamento (`UnavailableState` com retry).

## Critério de aceite

- `go test ./internal/timeline/... ./internal/httpapi/...`: `aggregate`/
  `build` (real-only) cobertos com fixture determinística (ver seção
  "Validação" do doc-guarda-chuva); teste HTTP de reconciliação: soma de
  `CategoryImpact[].Amount` do mês bate com a soma dos `Entry`s reais do
  mesmo mês/categoria.
- Frontend: `npm test` cobre `useTimeline`, contracts, e os componentes
  novos; navegação manual — alternar mês/ano no `/relatorio-financeiro` e
  verificar que "mês selecionado" e "acumulado no ano" nunca aparecem
  misturados visualmente.
