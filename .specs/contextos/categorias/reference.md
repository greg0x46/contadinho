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
`money.CategoryKind`, mais as decisões de categoria por transação
(`transaction_category_decisions`, histórico append-only em
`transaction_category_events`).

### Origem da decisão e precedência

Cada decisão carrega um `origin`; a precedência é
`manual > rule > learned > automatic`:

- `manual` — o usuário escolheu (`AssignManual`). Nunca é sobrescrita.
- `rule` — ação `set_category` de uma regra de automação (`ApplyRule`).
  Sobrescreve `learned` e `automatic`.
- `learned` — copiada da decisão **manual mais recente** feita numa
  transação semelhante (`ApplyLearned`, `learned.go`). Semelhante =
  mesma descrição normalizada (minúsculas, espaços colapsados, sufixo de
  parcela ` N/M` removido) e mesmo `movement_type`. Sobrescreve
  `automatic`; propaga para todas as parcelas de uma compra parcelada.
  Só vale para transações que chegam **depois** da decisão manual — uma
  decisão manual nunca reclassifica o passado.
- `automatic` — mapeamento do `source_category` do Pluggy
  (`ApplyAutomatic`) ou heurística de pagamento de fatura
  (`ApplyAutomaticCardPayment`). Só escreve quando não há decisão.

Ordem no sync (`syncsvc.upsertTransaction`, só em inserção): fatura de
cartão → mapeamento Pluggy → aprendida → regras (hook). Lançamento manual
sem `category_id` segue a mesma ordem (aprendida → regras).

## Rotas HTTP

`GET/POST /api/categories`, `PATCH /api/categories/{id}`.

## Frontend

`/categorias` — CRUD do catálogo.
