# Patrimônio Líquido (Net Worth)

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Sem spec de feature dedicado até o
> momento.

## O que é

Snapshots e histórico de patrimônio (ativos − passivos) ao longo do tempo.

## Estado vs. ideal

Não avaliado como candidato a motor de domínio em profundidade — ver
`.specs/motores-de-dominio.md` seção "Não são motores": não atende (por
ora) ao critério 3 (reuso em múltiplos contextos).

## Backend

`internal/networth` — cálculo/backfill de snapshots (`Breakdown`,
`SnapshotRow`).

O cálculo atual inclui posições e caixa de investimentos manuais via
`investments.ManualNetWorth`, além das posições importadas. Caixa de uma conta
de investimento vinculado a uma conta financeira existente não é somado de
novo. Carteiras por objetivo são agrupamentos e não acrescentam patrimônio.
Avaliações manuais são datadas; na ausência delas o valor é o custo informado.
O backfill continua sem inventar valorizações históricas dos investimentos.

## Rotas HTTP

`GET /api/net-worth`.

## Frontend

`/patrimonio-liquido` — card de composição + `NetWorthChart` (Recharts)
com a evolução ao longo do tempo.
