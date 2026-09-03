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

Podem existir várias conexões (um `data_sources` por item da Pluggy, ou seja,
um por banco). Todas usam o mesmo client ID/secret — são itens da mesma
aplicação Pluggy — e cada sync run pertence a exatamente uma conexão.

## Backend

- `internal/datasources` — registro de conexões: lista/cadastra/renomeia e
  ativa/desativa `data_sources`. Não há exclusão: desativar preserva contas,
  transações e histórico e apenas tira a conexão dos syncs futuros.
- `internal/pluggy` — adapter da API Pluggy (`Adapter`, `Config`, auth/
  retry), mapeia payloads brutos para `SourceSnapshot`/contas/investimentos/
  faturas. Sem rotas próprias.
- `internal/syncsvc` — `Service.Execute` orquestra uma execução de sync
  contra um `Provider` (o adapter Pluggy): processa contas, investimentos,
  faturas, registra imports brutos e falhas/rejeições.
- `internal/worker` — loop de polling em background (`ClaimNextRun`,
  `ProcessClaim`, `Run`). O item consultado vem do `source_id` da execução,
  não de configuração global. Assume uma única instância do worker rodando
  por vez — não coordena reivindicação de execuções entre múltiplos processos
  (relevante ao rodar várias instâncias contra um Postgres compartilhado, ver
  README raiz). Executa uma run por vez, então N conexões sincronizam em
  série.

## Rotas HTTP

`GET/POST /api/data-sources`, `GET/PATCH /api/data-sources/{id}`.

`POST/GET /api/sync-runs`, `GET /api/sync-runs/{id}`. O POST aceita
`source_id` opcional: sem ele, enfileira uma execução por conexão ativa
(pulando as que já estão sincronizando) e responde 202 com a lista do que
começou; só responde 409 quando nada pôde começar.

## Frontend

- `/open-banking` — conexões cadastradas (adicionar, renomear, ativar/
  desativar, sincronizar uma só) e lista/histórico de execuções de sync, com
  ação de sincronizar todas.
- `/open-banking/sync-runs/:id` — detalhe de uma execução, com polling
  enquanto ela está em andamento.

## Notas

- O item é criado no painel da Pluggy e colado aqui — não há suporte a
  connect token / widget do Pluggy Connect no adapter.
- Saldo de conta reportado pela Pluggy **não** é a referência de verdade do
  sistema — não considera regras de inclusão/exclusão de transações. Ver
  contexto de Transações.
