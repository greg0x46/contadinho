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
leitura a partir de `payable_transaction_links`, nunca persistidos.
`summary.go` é o único lugar onde essa recomputação vive: `Summarize`/
`SummarizeAsOf` (um payable), `SummarizeAll` (um conjunto, em duas queries
— é o que a listagem HTTP usa) e `RemainingTotal`/`RemainingTotalAsOf`/
`RemainingTotalsAsOf` (agregados). Antes a mesma sequência estava
reimplementada em quatro lugares, dois deles na camada HTTP. A matemática
pura (settled/remaining/status) é interna ao pacote: `Summarize` é a única
porta de entrada, para ninguém remontar as peças por fora.

`Summary` carrega os próprios vínculos (`Links []LinkSummary`, com valor
efetivo e os campos de exibição da transação) em vez de só uma contagem —
a tela de detalhe formata o que essa passagem já carregou, sem recarregar
cada vínculo e cada transação atrás dele. Sob `SummarizeAsOf` a lista traz
exatamente os vínculos que contavam naquele dia, então todo campo do
`Summary` descreve o mesmo momento.

Inclui também elegibilidade de vínculo (`eligibility.go`, hoje deriva a
direção de fluxo aceita a partir do `Kind` debt/receivable).

Os helpers de dívida/vencimento de cartão **saíram daqui** para
`internal/transactions` (motor de Lançamentos) — ver contexto de
Transações. A soma "dívida de payables + fatura do cartão" do widget
"Dívida total" não voltou para cá: ela junta dois motores e é composta no
handler (`handlePayableTotalOwed`), porque nenhum dos dois é dono do
número — e é ali também que a moeda BRL fica fixada.

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
