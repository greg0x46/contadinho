# Transações

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Motor de domínio por trás: motor de
> Lançamentos (seção 1 de `.specs/motores-de-dominio.md`).

## O que é

Lista pesquisável/filtrável de transações reais, com inclusão/exclusão
manual (ex.: ignorar um estorno ou duplicata) e categorização. Também
expõe contas, cartões e investimentos — dados adjacentes servidos pelos
mesmos handlers de apresentação.

Além das transações sincronizadas do Pluggy, um lançamento pode ser criado à
mão (`origin='manual'`) numa conta que já existe — mesma entidade, mesmas
regras de categorização/inclusão/automação, só outra via de entrada. Editar
e excluir só valem para um lançamento manual; um sincronizado nunca muda por
aqui. O saldo da conta continua vindo só do provedor — um lançamento manual
nunca o altera (ver Notas).

Categorizar como transferência entre contas próprias (`kind='transfer'`)
também tira o lançamento dos totais, com motivo próprio
(`transfer_category`) e sem marcá-lo como ignorado — ver a nota sobre as
duas vias em `.specs/motores-de-dominio.md` seção 1.

## Backend

- `internal/transactions` — `QueryRequest`/`Filters`/`Item`/`Page`/`Group`,
  mutação de inclusão/categoria, e `manual.go` (`CreateManual`/
  `UpdateManual`/`DeleteManual`) para a via de entrada manual.
- `internal/money` — as regras que decidem o que conta pra total:
  `Classify`, `SelectEffectiveMoney`, `Considered`/`Ignored`, e o
  `CategoryKind` `transfer`. Agnóstico de origem — Pluggy e lançamento
  manual populam a mesma tabela, e o motor em si não presume qual das duas.
  `Eligibility` recebe o kind da categoria atribuída; quem calcula *saldo*
  em vez de fluxo passa `""` para a regra de transferência não vazar
  (`networth/backfill.go`, `transactions/cardtotal.go`).
- `financial_transactions.origin` (`synced`/`manual`) distingue as duas
  vias; as colunas específicas de sync (`source_id`, `external_id`,
  `current_raw_import_id`, `normalized_hash`) ficam `NULL` numa linha
  manual — não existe `data_sources`/`raw_import` fake por trás. `deleted_at`
  é o soft-delete de um lançamento manual: `transaction_category_events`/
  `transaction_inclusion_events` são append-only e têm `ON DELETE RESTRICT`
  para `financial_transactions`, então uma linha manual já categorizada ou
  ignorada não pode ser apagada de verdade — `DeleteManual` marca
  `deleted_at` em vez disso, e `viewSelect` (a base de todo SELECT deste
  pacote, incluindo `GetItem`) filtra por `deleted_at IS NULL`. Um
  lançamento manual nunca entra em `networth.cashDeltasDescending`/
  `earliestCashTransactionDay` (filtro `origin = 'synced'`) — ver Notas.
- Cartão de crédito (`cardflow.go`/`cardtotal.go`, vindos de
  `internal/payables`): `CardDueDates`/`ProjectedEntryDate` (a data em que
  uma compra no cartão vira saída de caixa — o vencimento da fatura, não o
  `occurred_at`), `CreditAccountIDs`, `CardMetadataByTransaction` e
  `CreditCardTransactionTotal(At)` (o quanto se deve no ciclo aberto,
  calculado dos lançamentos elegíveis, nunca de `financial_accounts.balance`).
- `balance.go` — `CashOnHand`: dinheiro em conta (soma dos saldos
  reportados das contas não-crédito), a única implementação, usada pela
  Timeline e pelo Patrimônio Líquido. Ver a nota sobre saldo em
  `.specs/motores-de-dominio.md` seção 1 para por que uma transação
  ignorada continua dentro do saldo.

## Rotas HTTP

- `POST /api/transactions/query`, `GET /api/transactions/spending-by-category`,
  `PUT /api/transactions/{id}/inclusion`, `PUT /api/transactions/{id}/category`.
- Em `POST /api/transactions/query`, `filters.classification` aceita
  `inflow` (entrada, valor positivo), `outflow` (saída, valor negativo) e
  `unclassified`; a interface oferece só entradas e saídas. Qualquer outro
  valor é 400 (`invalid-classification`).
- Lançamento manual: `POST /api/transactions` (cria), `PUT
  /api/transactions/{id}` (edita), `DELETE /api/transactions/{id}` (exclui)
  — as três só aceitam uma transação com `origin='manual'` (409 caso
  contrário). A criação roda `automation.ApplyToNewTransaction` como uma
  sync rodaria, e depois aplica `category_id` explícito por cima, se vier —
  a escolha do usuário sempre vence o que a automação tiver decidido.
- Contas/cartões: `GET /api/accounts[/{id}][/cards|/bills]`,
  `PUT /api/accounts/{id}/closing-day`.
- Investimentos: `GET /api/investments[/{id}][/transactions]`.

## Frontend

- `/transacoes` — ledger principal: filtros, agrupamento, totais, drawer de
  edição de categoria/inclusão, botão "Novo lançamento manual" e — para um
  lançamento manual — tag "Manual" na lista e no drawer, com ações Editar/
  Excluir. `ManualTransactionForm` serve criação e edição.
- `/contas-e-cartoes` + `/contas-e-cartoes/:id`.
- `/investimentos` + `/investimentos/:id`.

## Notas

Investimentos conciliados acrescentam `investment_transfer_amount` e
`reportable_amount` ao lançamento. A parcela de aporte/resgate não entra nos
totais de receita/despesa, mas o valor integral continua sendo movimento de
caixa. O motivo de exclusão integral é `investment_transfer`. Ver
[`investimentos/reference.md`](../investimentos/reference.md).

Ingestão: Pluggy e lançamento manual (`.specs/lancamentos-manuais.md`) são
as duas vias de entrada hoje, ambas provedor de dado, não parte deste motor
— ver contexto de Sincronização Open Banking e `.specs/motores-de-
dominio.md` seção 1. Criar uma conta 100% manual (sem nenhum vínculo com
Pluggy) fica fora de escopo por ora — todo lançamento manual aponta para
uma conta que já existe.

O saldo de conta (`CashOnHand`, Patrimônio Líquido) nunca reflete um
lançamento manual — só o que o provedor reporta em
`financial_accounts.balance`. Um lançamento manual afeta o extrato e os
totais de receita/despesa, mas nunca o saldo exibido da conta nem a
reconstrução histórica de patrimônio (`networth.Backfill` filtra
`origin = 'synced'` explicitamente por isso). Cartão de crédito é a
exceção deliberada: `CreditCardTransactionTotal` já soma dos lançamentos
elegíveis (nunca do saldo reportado), então um lançamento manual numa
conta de crédito conta normalmente para a fatura em aberto.
