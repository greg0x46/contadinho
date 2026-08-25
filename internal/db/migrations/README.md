# Migrations

Two independent goose sequences, one per dialect. `internal/db` picks the
directory from the DSN and each database tracks its own version, so the two
sequences never meet at runtime.

## Os números não correspondem entre dialetos

Um mesmo passo lógico tem números diferentes em `sqlite/` e `postgres/`. Não é
descuido: as sequências divergiram cedo (o SQLite ganhou migrations que o
Postgres nunca precisou, e vice-versa — `00024_drop_stale_payables_receivable_column`
só existe no Postgres), e renumerar história quebraria qualquer banco já
migrado. Desde `00025` o Postgres está exatamente **um número atrás** do
SQLite para os mesmos passos:

| passo lógico | sqlite | postgres |
|---|---|---|
| `recurrence_reconciliations` | 00026 | 00025 |
| `scenario_projection_unification` | 00027 | 00026 |
| `drop_scenario_transaction_realizations` | 00028 | 00027 |
| `drop_recurrence_reconciliations` | 00029 | 00028 |
| `drop_recurring_commitments` | 00030 | 00029 |

**Ao adicionar uma migration:** case pelo *sufixo* (o nome do passo), nunca
pelo número. Se o passo vale para os dois bancos, crie os dois arquivos com o
mesmo sufixo e o próximo número **de cada** diretório.

## SQLite: `PRAGMA foreign_keys`

Um rebuild de tabela (`CREATE ... _new` / copiar / `DROP` / `RENAME`) só
precisa desligar as FKs quando **outra** tabela referencia a que está sendo
reconstruída, ou quando uma linha precisa apontar para outra que ainda não
existe. Fora esses casos, deixe as FKs ligadas e a migration dentro da
transação do goose — é o que dá rollback de verdade se algo falhar no meio.
`PRAGMA foreign_keys` é ignorado dentro de uma transação, então usá-lo obriga
a `-- +goose NO TRANSACTION`, e aí uma falha no meio deixa o schema pela
metade *e* a conexão do pool com FK desligada. Prefira reordenar os inserts
(ver `00030_drop_recurring_commitments.sql`) a pagar esse preço.
