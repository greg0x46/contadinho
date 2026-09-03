# Transações

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Motor de domínio por trás: motor de
> Lançamentos (seção 1 de `.specs/motores-de-dominio.md`).

## O que é

Lista pesquisável/filtrável de transações reais, com inclusão/exclusão
manual (ex.: ignorar um estorno ou duplicata) e categorização. Também
expõe contas, cartões e investimentos — dados adjacentes servidos pelos
mesmos handlers de apresentação.

Categorizar como transferência entre contas próprias (`kind='transfer'`)
também tira o lançamento dos totais, com motivo próprio
(`transfer_category`) e sem marcá-lo como ignorado — ver a nota sobre as
duas vias em `.specs/motores-de-dominio.md` seção 1.

## Backend

- `internal/transactions` — `QueryRequest`/`Filters`/`Item`/`Page`/`Group`,
  mutação de inclusão/categoria.
- `internal/money` — as regras que decidem o que conta pra total:
  `Classify`, `SelectEffectiveMoney`, `Considered`/`Ignored`, e o
  `CategoryKind` `transfer`. Agnóstico de origem (hoje só Pluggy popula,
  mas o motor em si não presume isso). `Eligibility` recebe o kind da
  categoria atribuída; quem calcula *saldo* em vez de fluxo passa `""` para
  a regra de transferência não vazar (`networth/backfill.go`,
  `transactions/cardtotal.go`).
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
- Contas/cartões: `GET /api/accounts[/{id}][/cards|/bills]`,
  `PUT /api/accounts/{id}/closing-day`.
- Investimentos: `GET /api/investments[/{id}][/transactions]`.

## Frontend

- `/transacoes` — ledger principal: filtros, agrupamento, totais, drawer de
  edição de categoria/inclusão.
- `/contas-e-cartoes` + `/contas-e-cartoes/:id`.
- `/investimentos` + `/investimentos/:id`.

## Notas

Ingestão (Pluggy hoje, lançamento manual no futuro) é provedor de dado, não
parte deste motor — ver contexto de Sincronização Open Banking e
`.specs/motores-de-dominio.md` seção 1.
