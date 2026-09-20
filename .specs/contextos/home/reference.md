# Home

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Motor de domínio por trás: Timeline, a
> fusão (seção 6 de `.specs/motores-de-dominio.md`).

## O que é

O painel inicial: saldo/entradas/saídas do período selecionado, a projeção
de saldo (hoje, fim do horizonte, menor saldo) e o gasto por categoria. Os
dois primeiros leem da mesma `Series` do backend, nunca recalculando
localmente; o que muda entre eles é o campo que cada um consome.

Já existiu uma tela de retrospectiva mensal (`/relatorio-financeiro`, com
quebra mês a mês, drill-down por categoria e comparativos); foi removida
junto com as agregações do `/api/timeline` que só ela consumia
(`monthly_breakdown`, `category_breakdown`, `month_over_month`,
`year_over_year`, `category_evolution`). Recuperá-las do histórico é o
caminho se a retrospectiva voltar.

## Backend

`internal/timeline` — `Series{Points, Entries, StartingBalance,
LowestBalance, FirstNegative}`, `Entry` com `CertaintyTier`
(`realizado`/`confirmado`/`projetado`/`hipotetico`). `BuildSeries` funde **2
fontes**: Lançamentos e o fluxo unificado de projeção de Cenários
(`projections.List`) — recorrência deixou de ser fonte paralela e virou
`Scenario{Kind: recurring}`. Os 4 tiers continuam todos produzidos:

- **Realizado** — transação real; um lançamento de cartão vira
  **Confirmado**, redatado para o vencimento da fatura.
- **Confirmado** — parcela de plano de payable (dívida real com data
  planejada).
- **Projetado** — ocorrência de recorrência **não reconciliada**. Uma
  ocorrência reconciliada contra uma transação real não gera entry: a
  transação real já carrega o dinheiro (antes gerava, e o valor era contado
  duas vezes). Os candidatos que uma ocorrência tenta reconciliar são
  escopados ao mês dela. Quem decide se a ocorrência está reconciliada é
  `recurrences.Reconciler`, o mesmo resolvedor que a tela de Recorrências
  lê — então desconciliar à mão faz a ocorrência voltar a projetar aqui, e
  conciliar à mão a suprime. Um vínculo manual pode apontar para uma
  transação do mês vizinho; a ocorrência é suprimida do mesmo jeito (a
  conciliação é um fato sobre a ocorrência), e a janela estreita de
  candidatas mantém o efeito limitado a uma virada de mês.
- **Hipotético** — transação de cenário standalone explicitamente
  selecionado.

O pacote **não conhece Payables**: o vínculo de um plano com um `Payable`
fica encapsulado em `internal/scenarios` (`Scenario.PayableID`/`Kind`, via
`ListPlanInstallments`/`SignedAmount`). O saldo âncora vem de
`transactions.CashOnHand`.

Cada `Entry` carrega duas leituras do mesmo dinheiro. A curva de saldo
(`Points`) usa `Amount` e inclui todo movimento que moveu caixa — o filtro
de `BuildSeries` é `MovesCash`, não `Included`. Entradas e saídas do
período (`TotalsForPeriod`) usam `ReportableAmount`, o valor reportável que
`transactions.toItem` zera para o que a elegibilidade de totais exclui.
Assim uma transferência entre contas próprias (categoria de transferência,
ou a parcela conciliada como aporte/resgate de investimento) desconta o
saldo mas fica fora de entradas e saídas.

## Rotas HTTP

`GET /api/timeline` — `reference_date`, `from`, `to`, filtros
(`account_ids`, `category_ids`, `card_numbers`) e `scenario_ids`. Devolve
`base`, `period_totals`, `simulation` e `scenario_impacts`.

`GET /api/timeline/range` — a janela mais larga que a curva cobre (da
transação mais antiga à última parcela planejada), usada pela opção "todo o
período".

## Frontend

Home (`/`):
- `PeriodBalanceCard` — saldo, entradas e saídas do período
  (`period_totals`).
- `ProjectionSummaryCard` — saldo hoje, saldo no fim do horizonte, menor
  saldo, mais `ProjectionTimeline`. Horizonte selecionável (fim do mês, 3,
  6, 12 meses; 3 por padrão).
- `SpendingByCategoryCard` — gasto por categoria, lido de
  `GET /api/transactions/category-breakdown` (contexto Transações).

## Notas

**Simulação de cenários não tem entrada na UI hoje.** `GET /api/timeline`
continua aceitando `scenario_ids` e devolvendo `simulation` e
`scenario_impacts` — o backend está inteiro —, mas nenhuma tela os
consome: a Home é deliberadamente só a base ("sem cenários hipotéticos").
Os componentes que faziam essa leitura (`ScenarioMultiSelect`,
`BaseVsSimulationCompare`, `ProjectionComposition`) foram removidos por
estarem mortos; recuperá-los do histórico é o caminho se a simulação
voltar para a tela.

Pelo mesmo motivo o `CertaintyTier` não aparece mais em lugar nenhum da
interface — o vocabulário de certeza (princípio 4 de
`.specs/motores-de-dominio.md`) vive só no backend hoje. O badge que o
exibia (`EntryOriginBadge`) e o mapa de rótulos que ele usava saíram junto.
