# Relatório Financeiro

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Motor de domínio por trás: Timeline, a
> fusão (seção 6 de `.specs/motores-de-dominio.md`).

## O que é

Navegação temporal única: Realizado → Hoje → Projeção Base → Simulação.
Responde quanto o usuário terá numa data futura, qual seu menor saldo até
lá, e como cenários hipotéticos mudam essa resposta — cards, gráfico e
drill-down todos lendo da mesma série, nunca recalculando localmente.

## Backend

`internal/timeline` — `Series{Points, Entries, StartingBalance,
LowestBalance, FirstNegative}`, `Entry` com `CertaintyTier`
(`realizado`/`confirmado`/`projetado`/`hipotetico`). `BuildSeries` funde 4
fontes hoje: transações reais, parcelas de plano de payable, ocorrências de
recorrência, e transações de cenário standalone ativo.

## Rotas HTTP

`GET /api/timeline`.

## Frontend

`/relatorio-financeiro`:
- `TimeNavigator` (navegação mensal) + `ScenarioMultiSelect` (seleção de
  cenários ativos, estado na URL via `filterUrl.ts`).
- Cards de saldo atual/projetado/mínimo.
- `ProjectionTimeline` (base × simulação, Recharts) e
  `BaseVsSimulationCompare`.
- Comparativos mês-a-mês/ano-a-ano (`ComparisonStatistic`).
- `MonthlyEvolutionChart`, `AccumulatedResultCard`.
- Drill-down por categoria: `CategoryImpactList` → `CategoryEvolutionChart`.
- `ProjectionComposition` — detalhamento de impacto por cenário, painel
  colapsável quando há `scenarioImpacts`.

## Notas

Se Recorrências passar a se construir a partir de Cenários (ver contexto
de Recorrências), a Timeline colapsa de 3 para 2 fontes.
