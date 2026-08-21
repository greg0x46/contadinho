# Ideia (não implementar ainda): Teto de gasto diário ("quanto posso gastar hoje")

> Discussão salva para retomar depois. Nenhuma implementação foi feita.
> Esse arquivo deve virar `.ideas/teto-gasto-diario.md` no repositório
> `contadinho-go` quando sair do plan mode.

## Motivação original

Mostrar ao usuário quanto ele pode gastar **hoje** sem comprometer o resto
do mês — não uma média fixa (orçamento ÷ dias do mês), mas um teto que sobe
quando os dias anteriores economizaram e desce quando estouraram, com um
freio contra quedas bruscas (compra grande não deve zerar o teto de um dia
pro outro).

## Versão 1: orçamento manual + amortecimento (desenhada, com dois bugs achados)

### Mecanismo
```
teto_bruto(dia N)     = (orçamento_efetivo_do_mês − gasto_até_ontem) / dias_restantes
teto_amortecido(N)    = teto_bruto(N), se for alta (sem limite)
                       = max(teto_bruto(N), teto_amortecido(N-1) * (1 - damping%)), se for queda
                       = 0 imediatamente (sem amortecer), se teto_bruto(N) <= 0
                         + expõe "estourado em R$ X" = -teto_bruto(N) * dias_restantes
```
"Gasto elegível" = soma de `money.SelectEffectiveMoney` (BRL) de transações
`Outflow` + `Considered` + categoria `kind='expense'`.

Sem estado por dia persistido — recalculado do zero a cada leitura, no
mesmo espírito de `internal/debts` (paid/remaining/status nunca gravados,
sempre recomputados dos links vigentes — ver `internal/debts/eligibility.go`,
`internal/debts/store.go`).

### Bug 1 — rollover ingênuo se autocancela/inverte
Comparar sempre o gasto real do mês anterior contra o **valor bruto atual**
do orçamento faz um rombo desaparecer (ou virar crédito fantasma) em ~2
meses: o mês seguinte já gasta dentro de um teto reduzido, mas seria
comparado de novo contra o valor não-reduzido.
Exemplo: orçamento R$3000, janeiro gasta R$4000 (estoura R$1000) → fevereiro
vira R$2000 de teto efetivo → se o usuário gastar certinho R$2000 em
fevereiro, março recalcula rollover = 3000 − 2000 = **+1000** (crédito
fantasma, quando na verdade só empatou).

### Bug 2 — rollover cumulativo "puro" (sem histórico) é instável a edições
Fix ingênuo pro bug 1: um saldo acumulado desde uma âncora (`Σ orçamento −
gasto_real` de cada mês). Mas se cada termo da soma usa o valor **atual**
do orçamento (já que a decisão foi "orçamento = valor único, sem
histórico"), editar o orçamento hoje recalcula retroativamente **todo o
passado acumulado**, sem limite de tempo — ao contrário do teto diário, que
só reabre os dias do mês corrente.

### Fix decidido (mas superado pela Versão 2 abaixo)
Snapshot mensal dentro da própria tabela `settings` (sem migration nova):
- `budget.monthly_amount` — valor atual/editável.
- `budget.monthly_amount.<YYYY-MM>` — snapshot congelado de cada mês,
  criado na 1ª leitura daquele mês; editar o orçamento no mês corrente
  atualiza as duas chaves juntas, meses passados nunca são tocados.
- `budget.first_configured_month` — âncora gravada uma única vez, pra não
  aplicar rollover retroativo a meses anteriores a ter orçamento configurado.
- Rollover cumulativo = `Σ (snapshot_do_mês(m) − gasto_real(m))` de
  `first_configured_month` até o mês anterior — estável, não se autocancela
  nem é distorcido por edições futuras.

## Versão 2 (proposta durante a discussão, não desenhada em detalhe): orçamento = saldo real

Em vez do usuário configurar um número manual ("orçamento mensal"), usar o
**saldo real das contas** (`financial_accounts.balance`, já sincronizado via
Pluggy) como base do teto:
```
teto_bruto(dia N) ≈ saldo_disponível_agora / dias_restantes
```

### Por que é melhor
- **Mata os dois bugs de rollover inteiros** — saldo real já carrega o
  histórico sozinho (não precisa de snapshot mensal, âncora, nem soma
  cumulativa).
- **Elimina a dependência de categorização correta** — transferência entre
  contas próprias se cancela sozinha ao somar os saldos de todas as contas
  (outflow de uma, inflow de outra, soma líquida zero), sem precisar
  filtrar por `kind='expense'`/`kind='transfer'`.
- Reflete timing real de caixa (aperta antes do pagamento do salário, afrouxa
  depois) em vez de uma média artificial.

### O que fica em aberto / mais complexo
- **`financial_accounts.balance` é só um snapshot** (atualizado a cada
  sync, `internal/syncsvc/service.go:197-230`), não uma série histórica —
  pra rodar o loop de amortecimento dia a dia dentro do mês corrente,
  precisa reconstruir a trajetória do saldo a partir das transações (saldo
  de hoje menos/mais os movimentos de cada dia), não é um valor direto.
- **`account_type`/`account_subtype` são texto cru vindo do Pluggy**
  (`internal/pluggy/mapping.go:204-208`, `payload["type"]`/`payload["subtype"]`),
  **sem enum validado neste repo** — nenhum `CHECK` constraint (diferente de
  `categories.kind`), nenhum lugar no código assume valores específicos hoje.
- **Nada expõe saldo de conta em nenhuma API ou tela hoje** — não existe
  handler de contas nem UI de contas no frontend. Essa feature seria a
  primeira a expor isso, não uma extensão de algo existente.

### Pergunta em aberto: quais contas entram na soma do "saldo disponível"?
Discutida, **não decidida** — três opções, com os tradeoffs de cada uma:

1. **Somar todas as contas** (corrente + poupança + investimento + cartão de
   crédito como dívida negativa). Matematicamente mais limpo — transferências
   e pagamento de fatura se cancelam sozinhos. Mas dinheiro guardado em
   poupança/investimento reduziria o teto de hoje tanto quanto dinheiro na
   conta corrente, o que pode não ser o que o usuário sente como "disponível".
2. **Só contas correntes** (excluir poupança/investimento/cartão). Mais fiel
   a "dinheiro realmente disponível pra gastar agora", mas exige distinguir
   tipos de conta a partir de `account_type`/`account_subtype` crus do
   Pluggy — sem enum validado, precisaria olhar dados reais de sync pra
   descobrir os valores exatos usados, ou then uma tela nova pro usuário
   marcar manualmente.
3. **Usuário escolhe manualmente quais contas contam**. Mais correto e
   flexível, mas exige criar uma tela de contas do zero (não existe nenhuma
   hoje) só para esse toggle — escopo bem maior que as outras duas opções.

## Peças técnicas reaproveitáveis (válidas pra qualquer uma das duas versões)

- `internal/money/money.go`: `Classify`, `SelectEffectiveMoney`,
  `Eligibility`, `CanonicalDecimal`.
- `internal/money/period.go`: `Date`, `PeriodFor` (`GroupDay`/`GroupMonth`)
  — bucketing por dia/mês local dado um timezone.
- `internal/debts/eligibility.go`, `internal/debts/store.go`: padrão de
  "recomputar tudo na leitura, nunca persistir valor derivado" a espelhar.
- `internal/settings/store.go`: `Get`/`Set` de chave-valor em texto plano
  (`encrypted=false, unlockKey=nil`), reusável pra qualquer config nova sem
  migration.
- `internal/httpapi/transactions_handlers.go` `handleSpendingByCategory`
  (~389-410): padrão de `?timezone=` por request + `time.LoadLocation`,
  sem timezone persistido no servidor — deve ser reusado igual pra "hoje".
- **Cuidado de performance confirmado**: `internal/transactions/query.go`'s
  `fetchAllViews` faz `SELECT *` sem `WHERE`, filtra tudo em Go — aceitável
  pro uso atual, mas essa feature não deve reusar esse padrão em loop (uma
  query por dia do mês); precisa de queries bounded por `WHERE occurred_at
  >= ? AND occurred_at < ?`, usando o índice existente
  `ix_financial_transactions_occurred_at_id_desc`.

## Próximos passos ao retomar
1. Decidir Versão 1 (orçamento manual, com o fix de snapshot mensal) vs.
   Versão 2 (saldo real) — Versão 2 é mais elegante mas tem escopo maior
   (expor saldo de conta é território novo no app).
2. Se for Versão 2: decidir o escopo de contas (uma das 3 opções acima) e
   olhar dados reais de sync do Pluggy pra saber os valores de
   `account_type`/`account_subtype` disponíveis antes de desenhar qualquer
   filtro automático.
