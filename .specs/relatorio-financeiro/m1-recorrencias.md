# M1 — Cadastro de Recorrências (`internal/recurrences`)

> Parte de `.specs/relatorio-financeiro.md`. Entrega sozinha: CRUD funcional
> de compromissos recorrentes, mesmo antes do relatório existir.

## Motivação

Salário, aluguel e assinaturas só aparecem no Contadinho depois de já terem
sido sincronizados pela Pluggy — não há como saber hoje "quanto terei em
dezembro" considerando esses valores. `RecurringCommitment` é um compromisso
cadastrado manualmente pelo usuário, independente de qualquer transação real
específica (mesmo racional de isolamento de `scenario_transactions` vs
`financial_transactions` — ver spec de Cenários).

## Modelo de dados

```sql
CREATE TABLE recurring_commitments (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (trim(name) <> ''),
    kind TEXT NOT NULL CHECK (kind IN ('income','expense')),
    amount TEXT NOT NULL,
    category_id TEXT NOT NULL REFERENCES categories (id),
    account_id TEXT REFERENCES financial_accounts (id),
    cadence TEXT NOT NULL CHECK (cadence IN ('monthly','annual')),
    day_of_month INTEGER NOT NULL CHECK (day_of_month BETWEEN 1 AND 31),
    month_of_year INTEGER CHECK (month_of_year BETWEEN 1 AND 12),
    start_date TEXT NOT NULL,
    end_date TEXT,
    is_active INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1)),
    logic_operator TEXT NOT NULL DEFAULT 'and' CHECK (logic_operator IN ('and', 'or')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (cadence != 'annual' OR month_of_year IS NOT NULL)
);
CREATE INDEX idx_recurring_commitments_category_id ON recurring_commitments (category_id);
CREATE INDEX idx_recurring_commitments_account_id ON recurring_commitments (account_id);

CREATE TABLE recurring_commitment_conditions (
    id TEXT PRIMARY KEY,
    recurring_commitment_id TEXT NOT NULL REFERENCES recurring_commitments (id) ON DELETE CASCADE,
    field TEXT NOT NULL CHECK (field IN ('description', 'card', 'account', 'amount', 'day_of_month')),
    operator TEXT NOT NULL CHECK (operator IN ('contains', 'equals', 'within_percent', 'near_day')),
    value TEXT NOT NULL CHECK (trim(value) <> ''),
    position INTEGER NOT NULL CHECK (position >= 0),
    UNIQUE (recurring_commitment_id, position)
);
```

### Regra de conciliação configurável (revisão do desenho original)

O desenho original desta spec descartava a tabela
`recurring_commitment_conditions` (espelhando `automation_rule_conditions`):
"nada em M1 precisa de condições de correspondência arbitrárias configuradas
pelo usuário" — as duas condições de reconciliação (tolerância de valor e
proximidade de dia) eram derivadas automaticamente dos campos do
compromisso, com `day_of_month` implicitamente obrigatório na reconciliação
via uma tolerância fixa de 3 dias.

Essa decisão foi revertida: `day_of_month` sendo sempre obrigatório também
*na regra de conciliação* — em vez de só como âncora de agendamento — não
fazia sentido para compromissos onde o dia real varia mês a mês (ex.:
salário pago em dias úteis). A tabela volta, agora reaproveitando de fato o
motor de M0 (`internal/rules`) inteiro, do mesmo jeito que
`automation_rule_conditions`: o usuário compõe livremente a lista de
condições (`logic_operator` + `[]Condition`) que decide se uma transação
real satisfaz uma ocorrência — incluindo nenhuma condição de dia, se não
fizer sentido para aquele compromisso.

Dois grupos de dados agora ficam explicitamente separados, inclusive na UI
(ver seção UI): os campos **descritivos** da recorrência (nome, valor,
categoria, conta, cadência, `day_of_month`/`month_of_year` — usados só para
*agendar* a ocorrência no calendário via `OccurrencesInRange`) e os campos
da **regra de conciliação** (`logic_operator` + `Conditions` — usados só
para *decidir* se uma transação real satisfaz aquela ocorrência via
`ResolveOccurrence`). `day_of_month` continua obrigatório no primeiro grupo
(não dá pra agendar "todo mês" sem um dia), mas nunca é implícito no
segundo — se o usuário quiser exigir proximidade de dia na conciliação, ele
adiciona uma condição `day_of_month`/`near_day` explicitamente.

Convenção de `Value` para `FieldAmount`/`FieldDayOfMonth` nas condições de
uma recorrência (diferente de `automation`, onde `Value` é sempre literal):
guarda-se **só a tolerância** (ex.: `"10"` = 10%, `"3"` = 3 dias) — a
referência (valor esperado / dia esperado) é sempre a da ocorrência sendo
resolvida, nunca um número fixo digitado pelo usuário. `internal/recurrences/match.go`
combina os dois em tempo de resolução, no formato
`"<referência>:<tolerância>"` que `internal/rules` espera.

## Contratos Go

```go
package recurrences

type Kind string
const (
    KindIncome  Kind = "income"
    KindExpense Kind = "expense"
)

type Cadence string
const (
    CadenceMonthly Cadence = "monthly"
    CadenceAnnual  Cadence = "annual"
)

type RecurringCommitment struct {
    ID                   string
    Name                 string
    Kind                 Kind
    Amount               decimal.Decimal // magnitude, sempre positivo
    CategoryID           string
    AccountID            *string
    Cadence              Cadence
    DayOfMonth           int  // 1-31, clampado ao gerar ocorrências — só agendamento, nunca implica condição de conciliação
    MonthOfYear          *int // obrigatório sse Cadence == CadenceAnnual
    StartDate            time.Time
    EndDate              *time.Time
    IsActive             bool
    LogicOperator        rules.LogicOperator
    Conditions           []rules.Condition // regra de conciliação — ver "Regra de conciliação configurável"
    CreatedAt, UpdatedAt time.Time
}

type Occurrence struct {
    Date           time.Time
    ExpectedAmount decimal.Decimal
}

// OccurrencesInRange is pure: clamps DayOfMonth ao último dia de meses
// curtos (dia 31 em fevereiro -> 28/29), respeita StartDate/EndDate, e para
// Cadence == CadenceAnnual só emite em MonthOfYear.
func OccurrencesInRange(commitment RecurringCommitment, from, to time.Time) []Occurrence

// ResolveOccurrence decide se occurrence já foi satisfeita por uma
// transação real dentre candidates (já filtradas por categoria/conta/mês
// pelo chamador via transactions.Query). Usa internal/rules.Matches com
// commitment.Conditions — condições FieldAmount/FieldDayOfMonth têm sua
// referência (valor/dia esperado da occurrence) combinada com a tolerância
// armazenada em tempo de resolução; as demais condições passam literais.
func ResolveOccurrence(occurrence Occurrence, commitment RecurringCommitment, candidates []transactions.Item) (matched *transactions.Item, ok bool)
```

`ResolveOccurrence` retorna o primeiro candidato que casar (ordenado por
data), mesmo critério "first match wins" já usado em
`automation.ApplyToNewTransaction`.

## Endpoints HTTP

Espelha `internal/httpapi/automation_handlers.go`:

```
GET    /api/recurring-commitments
POST   /api/recurring-commitments
PUT    /api/recurring-commitments/{id}
PATCH  /api/recurring-commitments/{id}   (is_active)
DELETE /api/recurring-commitments/{id}
```

## UI

Tela própria (`/recorrencias`), CRUD com o formulário dividido em duas
seções visuais:

- **Dados da recorrência**: nome, tipo (receita/despesa), valor, categoria
  (select existente), conta (opcional), cadência (mensal/anual), dia do mês
  — obrigatório — (+ mês do ano se anual), data de início, data de fim
  (opcional).
- **Regra de conciliação**: operador lógico (E/OU) + lista de condições,
  reaproveitando o mesmo componente de construtor de condições de
  `AutomationRuleForm` (`ConditionListEditor`, compartilhado). Campos
  `amount`/`day_of_month` aparecem como um único input de "tolerância"
  (sem expor a referência, que é implícita); os demais
  (description/card/account) como campo+operador+valor livre, igual à
  automação. Ao menos uma condição é exigida; nenhuma é obrigatória por
  padrão — a recorrência recém-criada vem com `amount within_percent 10` +
  `day_of_month near_day 3` pré-preenchidos como sugestão editável/removível.

Atalho "criar a partir desta transação" na tela de transações pré-preenche
nome/valor/categoria/dia a partir de uma transação existente (a regra de
conciliação fica com a sugestão padrão acima), sem persistir nenhum vínculo
de dados.

## Critério de aceite

- `go test ./internal/recurrences/...` cobre: clamp de dia em fevereiro,
  cadência anual, ocorrência com match dentro da tolerância, ocorrência sem
  nenhum candidato correspondente.
- CRUD completo testável via HTTP (criar, listar, editar, pausar, apagar).
- Navegação manual: criar um compromisso, editá-lo, pausar, apagar — sem
  depender do relatório (M3+) existir.
