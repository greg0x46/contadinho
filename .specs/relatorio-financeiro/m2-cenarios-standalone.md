# M2 — Cenários "E se" genéricos (`Scenario.Kind = "standalone"`)

> Parte de `.specs/relatorio-financeiro.md`. Entrega sozinha: usuário cria um
> cenário hipotético (viagem, novo emprego) sem precisar de uma dívida/
> recebível associada, mesmo antes do relatório consumir isso.

## Motivação

`internal/scenarios` hoje só modela planos de pagamento de um `Payable`
(`Kind: debt_plan | receivable_plan`, `PayableID` obrigatório — ver
`.specs/plano-pagamento-e-cenarios-projecao.md`). O pedido do usuário lista
exemplos de cenário que não são dívida nem recebível: fazer uma viagem,
trocar de emprego, receber um aumento, cancelar um gasto. Em vez de criar um
segundo conceito de cenário paralelo, `Scenario` ganha um terceiro `Kind`
(`standalone`) com `PayableID` nulo, reaproveitando a mesma entidade e as
mesmas tabelas (`scenarios`, `scenario_transactions`).

## Schema atual (confirmado em `internal/db/migrations/sqlite/00013_payables.sql:54-66`)

```sql
CREATE TABLE scenarios (
    id TEXT PRIMARY KEY, kind TEXT NOT NULL, name TEXT NOT NULL,
    payable_id TEXT REFERENCES payables (id) ON DELETE CASCADE,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
    CHECK (payable_id IS NOT NULL)
);
```

## Migração

Nova migration `internal/db/migrations/sqlite/00015_scenarios_standalone.sql`
(+ `internal/db/migrations/postgres/00013_scenarios_standalone.sql` — M1 já
consumiu sqlite `00014`/postgres `00012` para `recurring_commitments`),
usando a mesma técnica de rebuild-por-rename já usada em `00013_payables.sql`
(SQLite não suporta `DROP CONSTRAINT`/`ALTER ... CHECK`):

```sql
CREATE TABLE scenarios_new (
    id TEXT PRIMARY KEY, kind TEXT NOT NULL, name TEXT NOT NULL,
    payable_id TEXT REFERENCES payables (id) ON DELETE CASCADE,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
    CHECK (
        (kind = 'standalone' AND payable_id IS NULL) OR
        (kind IN ('debt_plan', 'receivable_plan') AND payable_id IS NOT NULL)
    )
);
INSERT INTO scenarios_new SELECT * FROM scenarios;
DROP TABLE scenarios;
ALTER TABLE scenarios_new RENAME TO scenarios;
```

## Contratos Go

`internal/scenarios/model.go`:

```go
const (
    KindDebtPlan       Kind = "debt_plan"
    KindReceivablePlan Kind = "receivable_plan"
    KindStandalone     Kind = "standalone" // novo: sem payable_id
)
```

O comentário do tipo `Kind` (hoje afirma que "what_if" era non-goal) precisa
ser atualizado para refletir que standalone passou a ser implementado.
`Scenario.PayableID` já é `*string` — sem mudança de tipo, só passa a
aceitar `nil` também para `KindStandalone` (antes só dívida/recebível
tinham motivo de existir sem CHECK barrando nulo).

`internal/scenarios/store.go` ganha:

```go
// ListStandaloneScenarios lista todo Scenario{Kind: standalone}, sem
// relação a nenhum payable.
func ListStandaloneScenarios(ctx context.Context, q Querier) ([]Scenario, error)

// ListScenariosByIDs busca um lote de cenários por id — usado pelo
// multi-select do M5 para carregar só os cenários ativos na simulação.
func ListScenariosByIDs(ctx context.Context, q Querier, ids []string) ([]Scenario, error)
```

## Endpoints HTTP

Hoje só existe `POST /api/payables/{id}/scenarios` (cenário preso a um
payable). M2 adiciona, em `internal/httpapi/scenarios_handlers.go`:

```
GET  /api/scenarios?kind=standalone   -- lista cenários standalone
POST /api/scenarios                   -- cria cenário standalone (sem payable_id)
```

O `POST` existente (`/api/payables/{id}/scenarios`) continua exatamente como
está — nenhuma rota de dívida/recebível muda. O CRUD de
`ScenarioTransaction` (`POST/PUT/DELETE /api/scenarios/{id}/transactions...`)
já é genérico o suficiente para reaproveitar sem alteração — só precisa
funcionar também quando o `Scenario` pai é `standalone`.

## Frontend

- `frontend/src/api/scenarios.ts`: adicionar `listStandaloneScenarios()` e
  `createStandaloneScenario(name)`. CRUD de `ScenarioTransaction` já
  existente é reaproveitado sem mudança.
- `frontend/src/api/contracts.ts`: estender `scenarioKinds` com
  `"standalone"`.
- `frontend/src/hooks/useScenarios.ts` — **novo hook plural** (hoje só
  existe `usePayablePlan`, singular, que assume "o primeiro cenário do
  payable" como *o* plano). `useScenarios({ kind: "standalone" })` alimenta
  o multi-select do M5.
- Formulário simples de criação de cenário standalone (nome + transações
  hipotéticas), reaproveitando o CRUD de `ScenarioTransaction` já existente
  na tela de plano de pagamento — não duplicar componente, extrair o que for
  comum se necessário.

## Critério de aceite

- Migration roda sem perder dados de `debt_plan`/`receivable_plan`
  existentes (teste de migração, mesmo padrão de
  `internal/db/migration_data_test.go`).
- `go test ./internal/scenarios/... ./internal/httpapi/...` cobre: criar
  cenário standalone sem payable, listar só standalone, `POST` continua
  rejeitando `debt_plan`/`receivable_plan` sem `payable_id`.
- Navegação manual: criar um cenário "Viagem" com 2-3 transações
  hipotéticas, sem vincular a nenhuma dívida/recebível.
