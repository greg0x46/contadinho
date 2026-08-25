# Relatório Financeiro

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Motor de domínio por trás: Timeline, a
> fusão (seção 6 de `.specs/motores-de-dominio.md`).

## O que é

Retrospectiva mensal: quanto entrou, quanto saiu e onde o usuário mais
gastou no mês selecionado, e como isso se acumula no ano — cards, gráfico e
drill-down todos lendo da mesma série, nunca recalculando localmente.

O **futuro** saiu desta tela: a projeção de saldo vive na Home
(`ProjectionSummaryCard`), e a edição de cenários em Cenários. A `Series`
por trás das duas é a mesma; o que muda é a janela que cada tela pede.

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

## Rotas HTTP

`GET /api/timeline`.

## Frontend

`/relatorio-financeiro`:
- `TimeNavigator` (navegação mensal, mês na URL via `?mes=`).
- `SummaryCards` (mês selecionado × acumulado no ano).
- Comparativos mês-a-mês/ano-a-ano (`ComparisonStatistic`, local à página).
- `AccumulatedResultCard`.
- Drill-down por categoria: `CategoryImpactList` → `CategoryEvolutionChart`.

Home (`/`):
- `ProjectionSummaryCard` — saldo hoje, saldo no fim do horizonte, menor
  saldo, mais `ProjectionTimeline`. Horizonte selecionável (fim do mês, 3,
  6, 12 meses; 3 por padrão).

## Notas

A Timeline já colapsou de 3 para 2 fontes: Recorrências passou a se
construir a partir de Cenários, como a nota antiga previa.

**Simulação de cenários não tem entrada na UI hoje.** `GET /api/timeline`
continua aceitando `scenario_ids` e devolvendo `simulation` e
`scenario_impacts` — o backend está inteiro —, mas nenhuma tela os
consome desde que a projeção migrou para a Home, que é deliberadamente só a
base ("sem cenários hipotéticos"). Os componentes que faziam essa leitura
(`ScenarioMultiSelect`, `BaseVsSimulationCompare`, `ProjectionComposition`)
foram removidos por estarem mortos; recuperá-los do histórico é o caminho
se a simulação voltar para a tela.

Pelo mesmo motivo o `CertaintyTier` não aparece mais em lugar nenhum da
interface — o vocabulário de certeza (princípio 4 de
`.specs/motores-de-dominio.md`) vive só no backend hoje. O badge que o
exibia (`EntryOriginBadge`) e o mapa de rótulos que ele usava saíram junto.
