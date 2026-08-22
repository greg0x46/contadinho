# Cenários

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Motor de domínio por trás: Cenários, a
> camada hipotética (seção 5 de `.specs/motores-de-dominio.md`).

## O que é

Projeções autoradas pelo usuário, nunca fato sincronizado. Duas formas,
mesma entidade (`Scenario`/`ScenarioTransaction`): presa a um `Payable`
(plano de parcelas de uma dívida/recebível específica) ou livre
(`standalone`, "e se" genérico sem `Payable` nenhum — ex.: viagem, novo
emprego). Um cenário nunca contamina totais reais por padrão — só entra
num relatório quando o chamador passa `scenario_id` explicitamente.

## Estado vs. ideal

Totalmente implementado como descrito: `Kind` cobre `debt_plan`,
`receivable_plan` e `standalone`; `PayableID` é nulo para `standalone` e
obrigatório para os outros dois (checado por constraint no banco e na
camada HTTP).

## Backend

`internal/scenarios`. Além do CRUD: `SignedAmount` (a direção de caixa de
uma parcela, derivada do `Kind` do próprio cenário — é o que permite a
Timeline projetar um plano sem conhecer `payables`), `ListPlanInstallments`
(parcelas não realizadas de todo plano num intervalo) e `Summarize`/
`SummarizeTransaction` (status por parcela e desvio acumulado, antes
calculados no handler HTTP).

## Rotas HTTP

`GET/POST /api/scenarios`, `GET/DELETE /api/scenarios/{id}`,
`POST .../generate-installments`, `POST .../readjust`,
`POST/PUT/DELETE .../transactions[/{transactionId}]`,
`POST/DELETE .../transactions/{transactionId}/realizations[/{realizationId}]`.

## Frontend

`/cenarios` — "Simule decisões hipotéticas" (cenários standalone). O plano
de pagamento de uma dívida/recebível específica vive embutido em
`/pendencias/:id` (ver contexto de Pendências).
