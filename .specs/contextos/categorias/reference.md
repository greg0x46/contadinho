# Categorias

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature.

## O que é

Catálogo de categorias definido pelo usuário, com histórico de
categorização. Não é um motor de domínio — catálogo passivo, sem lógica de
match/cálculo/fusão própria (ver critério 2 e seção "Não são motores" em
`.specs/motores-de-dominio.md`).

## Backend

`internal/categories` — CRUD simples (`Create/Update/Get/List`) sobre
`money.CategoryKind`.

## Rotas HTTP

`GET/POST /api/categories`, `PATCH /api/categories/{id}`.

## Frontend

`/categorias` — CRUD do catálogo.
