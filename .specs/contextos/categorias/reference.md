# Categorias

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature.

## O que é

Catálogo de categorias definido pelo usuário, com histórico de
categorização. Não é um motor de domínio — catálogo passivo, sem lógica de
match/cálculo/fusão própria (ver critério 2 e seção "Não são motores" em
`.specs/motores-de-dominio.md`).

Uma exceção ao "passivo": o `kind` `transfer` carrega regra junto. Uma
transação com categoria desse kind fica **fora dos totais de
receita/despesa** (motivo `transfer_category`), porque contar as duas
pontas de uma transferência somaria o mesmo dinheiro duas vezes. Não
afeta saldo, patrimônio histórico nem dívida de cartão. `expense`/`income`
seguem sendo só rótulo. A semente traz uma categoria desse kind
(`Transferência entre Contas Próprias`, `00004_categories.sql`); o usuário
pode criar outras.

## Backend

`internal/categories` — CRUD simples (`Create/Update/Get/List`) sobre
`money.CategoryKind`.

## Rotas HTTP

`GET/POST /api/categories`, `PATCH /api/categories/{id}`.

## Frontend

`/categorias` — CRUD do catálogo.
