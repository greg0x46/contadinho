# Transações

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Motor de domínio por trás: motor de
> Lançamentos (seção 1 de `.specs/motores-de-dominio.md`).

## O que é

Lista pesquisável/filtrável de transações reais, com inclusão/exclusão
manual (ex.: ignorar um estorno ou duplicata) e categorização. Também
expõe contas, cartões e investimentos — dados adjacentes servidos pelos
mesmos handlers de apresentação.

## Backend

- `internal/transactions` — `QueryRequest`/`Filters`/`Item`/`Page`/`Group`,
  mutação de inclusão/categoria.
- `internal/money` — as regras que decidem o que conta pra total:
  `Classify`, `SelectEffectiveMoney`, `Considered`/`Ignored`. Agnóstico de
  origem (hoje só Pluggy popula, mas o motor em si não presume isso).

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
