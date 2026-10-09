# Invariantes financeiros (matriz de decisão e propriedade)

Doc **vivo** e transversal: descreve o comportamento **atual** das regras
financeiras centrais, qual módulo é dono de cada uma e qual teste a fixa.
Não é um doc de design — o porquê de cada motor está em
[`motores-de-dominio.md`](motores-de-dominio.md), e o detalhe por feature nos
`contextos/<contexto>/reference.md` (em especial
[`transacoes`](contextos/transacoes/reference.md),
[`pendencias`](contextos/pendencias/reference.md),
[`cenarios`](contextos/cenarios/reference.md),
[`investimentos`](contextos/investimentos/reference.md) e
[`patrimonio-liquido`](contextos/patrimonio-liquido/reference.md)).

Os testes transversais (que cruzam mais de um motor) ficam em
`internal/invariants`, um pacote só de testes. `TestMatrixReferencesExistingTests`
falha se esta matriz citar um teste que não existe — ao renomear um teste,
atualize a linha correspondente.

Quando uma mudança alterar qualquer regra abaixo, atualize a linha no mesmo
PR. Uma divergência encontrada e **não** corrigida vai para a seção
"Divergências" no fim, nunca é "consertada" de passagem.

## Matriz

| Conceito | Regra (invariante) | Comportamento atual | Módulo dono | Teste que fixa |
| --- | --- | --- | --- | --- |
| Saldo de conta | O saldo reportado pelo provedor é a referência de caixa; categorizar, ignorar ou marcar como transferência nunca o altera. Conta `CREDIT` não é caixa. | Soma de `financial_accounts.balance` das contas não-crédito. É a âncora de hoje da Timeline e o caixa do Patrimônio Líquido. | `transactions.CashOnHand` (`internal/transactions/balance.go`) | `TestCategorizationDoesNotAlterProviderBalances`, `TestCashOnHandExcludesCreditAccounts`, `TestCashOnHandScopedToAccountIDsFiltersUntypedAccounts` |
| Reconstrução histórica de saldo | Dias passados saem da âncora de hoje revertendo o que moveu dinheiro; transferência conta como movimento de caixa. | Timeline caminha só itens com `MovesCash()` (transferência sim, ignorado não). O backfill de patrimônio reverte todo lançamento elegível passando `Considered` e kind `""` — inclusive ignorados (ver Divergências). | `timeline.BuildSeries`/`buildPoints`, `networth.Backfill` (`cashDeltasDescending`), `money.MovedCash` | `TestBuildSeriesKeepsTransferCategorizedCashMovement`, `TestBuildSeriesStillDropsIgnoredTransactions`, `TestBackfillReversesTransferCategorizedTransactions`, `TestIgnoredRowDivergesBetweenTimelineAndBackfill` |
| Receita e despesa | Só entra no total o lançamento elegível; o primeiro motivo de exclusão é o reportado. Ordem: `ignored` > `unclassified` > `ineligible_status` > `missing_money_pair` > `zero_value` > `transfer_category`. | `POSTED` e `PENDING` são elegíveis. A ordem é load-bearing: `transfer_category` vem por último porque `MovedCash` o lê como "o dinheiro saiu". | `money.Eligibility`, `money.MovedCash`, totais de `transactions.Query` | `TestEligibility`, `TestEligibilityPrecedenceMatrix` |
| Transferência | Transferência entre contas próprias não é receita nem despesa, mas move caixa nas duas pernas. | Categoria de kind `transfer` gera `ReasonTransferCategory`; fora dos totais, dentro do caixa. A dívida de cartão continua abatendo pagamento de fatura categorizado como transferência. | `money.Eligibility`, `transactions.Query`, `transactions.SpendingByCategory`, `transactions.CreditCardTransactionTotal` | `TestTransfersCreateNoIncomeOrExpense`, `TestQueryExcludesTransferCategoryFromTotals`, `TestQueryExcludesAutomaticSamePersonTransferFromTotals`, `TestSpendingByCategoryExcludesTransfers`, `TestBuildSeriesTransferMovesCashButIsNeitherIncomeNorExpense`, `TestBuildSeriesDropsAnUnsettledTransfer`, `TestBuildSeriesDropsAnUnclassifiedTransfer`, `TestCreditCardTotalIgnoresTransferCategory` |
| Ignorado | Ignorado vence a transferência na precedência e não é descontado do saldo reportado. Ao ignorar, vínculos que dependiam do lançamento caem. | `SetInclusion(ignored)` dispara o `OnIgnoredHook`: `payables.UnlinkIfPresent` (vínculo + settlement/allocation) e, no caminho de investimentos, `investments.UnlinkTransactionIfPresent`. Uma linha ignorada não pode ser conciliada. | `transactions.SetInclusion`, `payables.UnlinkIfPresent`, `investments.UnlinkTransactionIfPresent` | `TestUnlinkIfPresentTriggeredByIgnoringTransaction`, `TestSetInclusionInvokesOnIgnoredHook`, `TestIgnoredBankLineCannotBeReconciled`, `TestUnlinkTransactionDropsEveryParcelAndTheOrphanPivot`, `TestObligationCannotSettleTwice` |
| Obrigação (payable) | Um lançamento quita no máximo uma obrigação, uma vez. Quitado/restante são recalculados na leitura, nunca gravados, e o restante fica em `[0, total]`. | `payables.EligibilityForLink` + `CreateLink` (conflito se já vinculado; UNIQUE em `payable_transaction_links.transaction_id`). Só settlements do cenário fonte contábil contam em `Summarize`. | `payables.CreateLink`, `payables.Summarize` | `TestObligationCannotSettleTwice`, `TestCreateLinkConflictsOnAlreadyLinked`, `TestPayableTransactionLinksUniqueAcrossKinds`, `TestRemainingAmountClampedToZero`, `TestRemainingAmountClampedToTotal` |
| Realização de plano/recorrência | Um lançamento realiza no máximo um evento completo (settlement ou conciliação de recorrência). | `scenarios.RealizeEvent` devolve `ErrTransactionAlreadyRealized` ao bater no índice `uq_scenario_realizations_one_complete_event_transaction`. Alocações (`allocation`) não entram nesse índice. | `scenarios.RealizeEvent` | `TestSameTransactionCannotReconcileTwoOccurrences`, `TestObligationCannotSettleTwice` |
| Investimento | Principal conciliado (aporte/resgate) não é receita nem despesa, mas o caixa do banco é preservado. Rendimento, imposto e custo contam. | A parte conciliada vira `ReasonInvestmentTransfer` e sai do `ReportableAmount`; só o restante não conciliado conta. Rendimento/imposto/custo manuais entram via `investments.ManualReportingEntries`, sem mover o caixa; a parte já conciliada a uma linha do banco não é contada de novo. | `investments/reconciliations.go`, `investments.ReconciledTransactionAmounts`, `transactions/query.go` (`applyInvestmentTransfer`), `timeline.BuildSeries` | `TestInvestmentPrincipalIsNotIncome`, `TestInvestmentAllocationSeparatesReportingFromCash`, `TestReconciledInvestmentSplitFlowsThroughQueryAndCategories`, `TestBankLineSplitsBetweenOperations` |
| Hipotético | Movimento de cenário avulso nunca vira lançamento real nem altera saldo. | Cenário `standalone` nasce inativo; entra só por seleção ativa ou `ScenarioIDs` explícito, como `TierHipotetico`/`SourceScenario`. Seleção explícita vazia não emite nada. Evento vinculado a um lançamento real é suprimido (sem dupla contagem). | `projections.List`, `timeline.BuildSeries`, `scenarios.CreateScenario` | `TestHypotheticalMovementsAreNotRealized`, `TestListSelectionAndEventKeys`, `TestListReturnsUnifiedKindsAndRealizationState` |
| Reprocessamento | Rodar o mesmo processamento de novo não duplica efeito financeiro. | Resync com hash igual é no-op; `SetInclusion` idempotente; automação reaplicada só conta mudanças reais e nunca sobrepõe decisão manual; `ApplyAutomatic` só grava sem decisão prévia; snapshot do dia é upsert de uma linha; backfill nunca sobrescreve; vínculo e realização protegidos por UNIQUE; conciliação repetida é recusada. | `syncsvc`, `transactions.SetInclusion`, `automation`, `categories`, `networth.Snapshot`/`Backfill`, `payables.CreateLink`, `investments.CreateReconciliation` | `TestRepeatedProcessingDoesNotDuplicateFinancialEffects`, `TestExecuteSecondRunDetectsUnchangedRecords`, `TestExecuteSecondRunDetectsUnchangedInvestment`, `TestExecuteResyncNeverOverridesManualInclusionDecision`, `TestSetInclusionIsIdempotent`, `TestApplyRetroactivelyCountsMatchedAndActuallyChanged`, `TestSnapshotIsIdempotentForTheSameDay`, `TestBackfillNeverOverwritesAnExistingSnapshot` |

## Divergências e mudanças pretendidas (não implementadas)

Nada aqui foi alterado. Cada item está fixado como comportamento atual e
fica para uma issue própria.

1. **Sinal da parcela de plano** (nota aberta 5 de
   `motores-de-dominio.md`): `scenarios.SignedAmount` nega o valor de um
   `debt_plan` em vez de forçar a direção; uma parcela gravada com valor
   negativo lê como entrada.
2. **Ignorado no passado: Timeline × backfill de patrimônio.** A Timeline
   descarta o lançamento ignorado da caminhada (duplicata/estorno), e o
   backfill o reverte da âncora (dinheiro que saiu de verdade). O saldo de
   hoje é o mesmo nos dois; nos dias anteriores ao lançamento os dois gráficos diferem
   pelo valor dele (`TestIgnoredRowDivergesBetweenTimelineAndBackfill`). O
   comentário de cabeçalho de `networth.Backfill` ainda diz que ignorado
   "nunca é revertido", o que contradiz o código e o comentário de
   `cashDeltasDescending`.
3. **`PENDING` é elegível.** Um lançamento pendente conta nos totais e, se
   for transferência, como movimento de caixa — antes de o provedor
   liquidá-lo (`TestTransfersCreateNoIncomeOrExpense`, caso
   "pending transfer is still a transfer").
4. **Alocação não é única por lançamento.** O índice de evento completo
   cobre só `settlement` e `reconciliation`; um mesmo lançamento pode ser
   alocado a mais de uma parcela de cenário, e `RealizeEvent` não confere a
   soma das alocações contra o valor do lançamento. Um pagamento de 800
   suprime hoje duas parcelas de 800 (`TestHypotheticalMovementsAreNotRealized`).
