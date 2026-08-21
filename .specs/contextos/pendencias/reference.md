# Pendências (dívidas e recebíveis)

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Motor de domínio por trás: Payables
> (seção 4 de `.specs/motores-de-dominio.md`).

## O que é

Rastreia um alvo real: um total (dívida ou recebível) que vai sendo
abatido por transações reais vinculadas, com settled/remaining/status
sempre recomputados na leitura, nunca persistidos.

## Estado vs. ideal

O pacote `internal/payables` substituiu por completo o antigo
`internal/debts` (que não existe mais no código). `Kind` hoje ainda só
cobre `debt`/`receivable` — a generalização para incluir metas ("juntar
R$10.000 até dezembro"), descrita como direção em
`.specs/motores-de-dominio.md` seção 4, **não foi implementada**. Nome
"Payables" já é considerado provisório mesmo antes dessa generalização
(ver nota de nome no mesmo doc).

## Backend

`internal/payables` — total/quitado/restante sempre recomputados na
leitura a partir de `payable_transaction_links`, nunca persistidos. Inclui
elegibilidade de vínculo (`eligibility.go`, hoje deriva a direção de fluxo
aceita a partir do `Kind` debt/receivable) e helpers de dívida de cartão.

## Rotas HTTP

`GET/POST /api/payables`, `GET/PUT/DELETE /api/payables/{id}`,
`GET /api/payables/total-owed`, `/total-to-receive`,
`/eligible-transactions`, `POST /api/payables/{id}/links`,
`DELETE .../links/{linkId}`, mais rotas de cenário escopadas por payable —
`GET/POST /api/payables/{id}/scenarios` (ver contexto de Cenários).

## Frontend

`/pendencias` — lista combinada dívidas/recebíveis.
`/pendencias/:id` — detalhe, incluindo timeline e o plano de pagamento
(cenário) associado.
