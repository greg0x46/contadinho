# Sincronização Open Banking

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Para o racional/histórico de decisões,
> não há spec de feature dedicado (a integração Pluggy é a base histórica
> do projeto). Para o papel deste contexto na arquitetura de domínio, ver
> `.specs/motores-de-dominio.md` — não é ele mesmo um motor de domínio
> (é provedor de ingestão para o motor de Lançamentos), ver a seção
> "Não são motores" lá.

## O que é

Um worker em background consulta a Pluggy periodicamente em busca de novas
transações e dados de conta, registrando cada execução como um "sync run"
auditável (histórico, métricas, falhas) — não uma importação caixa-preta.

## Backend

- `internal/pluggy` — adapter da API Pluggy (`Adapter`, `Config`, auth/
  retry), mapeia payloads brutos para `SourceSnapshot`/contas/investimentos/
  faturas. Sem rotas próprias.
- `internal/syncsvc` — `Service.Execute` orquestra uma execução de sync
  contra um `Provider` (o adapter Pluggy): processa contas, investimentos,
  faturas, registra imports brutos e falhas/rejeições.
- `internal/worker` — loop de polling em background (`ClaimNextRun`,
  `ProcessClaim`, `Run`). Assume uma única instância do worker rodando por
  vez — não coordena reivindicação de execuções entre múltiplos processos
  (relevante ao rodar várias instâncias contra um Postgres compartilhado,
  ver README raiz).

## Rotas HTTP

`POST/GET /api/sync-runs`, `GET /api/sync-runs/{id}`.

## Frontend

- `/open-banking` — lista/histórico de execuções de sync, com ação de
  disparar um novo sync.
- `/open-banking/sync-runs/:id` — detalhe de uma execução, com polling
  enquanto ela está em andamento.

## Notas

- Saldo de conta reportado pela Pluggy **não** é a referência de verdade do
  sistema — não considera regras de inclusão/exclusão de transações. Ver
  contexto de Transações.
