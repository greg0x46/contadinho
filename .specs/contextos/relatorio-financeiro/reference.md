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
(`realizado`/`confirmado`/`projetado`/`hipotetico`). `BuildSeries` funde 3
fontes: Lançamentos, Cenários e Recorrências. Os 4 tiers são todos
produzidos hoje:

- **Realizado** — transação real; um lançamento de cartão vira
  **Confirmado**, redatado para o vencimento da fatura.
- **Confirmado** — parcela de plano de payable (dívida real com data
  planejada).
- **Projetado** — ocorrência de recorrência **não reconciliada**. Uma
  ocorrência reconciliada contra uma transação real não gera entry: a
  transação real já carrega o dinheiro (antes gerava, e o valor era contado
  duas vezes). Os candidatos que uma ocorrência tenta reconciliar são
  escopados ao mês dela.
- **Hipotético** — transação de cenário standalone explicitamente
  selecionado.

O pacote **não conhece Payables**: o vínculo de um plano com um `Payable`
fica encapsulado em `internal/scenarios` (`Scenario.PayableID`/`Kind`, via
`ListPlanInstallments`/`SignedAmount`). O saldo âncora vem de
`transactions.CashOnHand`.

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
