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
  Cada lançamento usa uma transação de banco que inclui o upsert, o hash,
  as categorias, o hook de automação, os eventos de auditoria e os contadores.
  Se qualquer etapa falhar, todo o registro é revertido e a rejeição é
  registrada separadamente; os demais registros da página continuam.
  Uma nova sync com o mesmo payload tenta novamente a inserção/atualização,
  pois a falha não confirmou o novo hash. Registros concluídos com hash
  igual continuam `unchanged`, sem reaplicar categorias ou automação.
- `internal/worker` — loop de polling em background (`ClaimNextRun`,
  `ProcessClaim`, `Run`). O item consultado vem do `source_id` da execução,
  não de configuração global. Executa uma run por vez por instância, então
  N conexões sincronizam em série.
  - Reivindicação atômica: um único `UPDATE ... WHERE worker_id IS NULL
    ... RETURNING` marca `worker_id`/`heartbeat_at`; com várias instâncias no
    mesmo Postgres, exatamente uma vence cada run.
  - Lease: o dono atualiza `heartbeat_at` a cada 15s; se o heartbeat não
    encontra mais a run como sua, a execução é cancelada
    (`sync_run_claim_lost`).
  - Gravações restritas ao dono: importações brutas e cada transação de
    persistência verificam a posse da run na mesma transação que grava os
    dados. `finalize`, `failGeneral` e `FailRun` também exigem o dono;
    caso contrário retornam `syncsvc.ErrClaimLost`. Run já terminal é no-op
    (sem conclusão dupla).
  - Recuperação: no início e a cada ~45s, independentemente da execução de
    outra sync, runs reivindicadas sem heartbeat
    há 90s viram `failed` com `general_error_code = 'interrupted'`, por
    compare-and-set (uma única falha registrada mesmo com recuperações
    concorrentes). Não há retry automático; o agendamento diário ou
    "Sincronizar agora" cria uma nova. Runs ainda não reivindicadas
    sobrevivem ao restart e são executadas depois. Com uma instância, a run
    interrompida por crash só é recuperada quando o lease expira (até ~90s
    após o restart), e até lá a conexão aparece como sincronizando.
  - Limites: SQLite atende um único processo (uma conexão; compartilhar o
    arquivo entre processos não é suportado). Múltiplas instâncias exigem
    Postgres.

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
