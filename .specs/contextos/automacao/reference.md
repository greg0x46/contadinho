# Automação

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Motor de domínio por trás: Automação,
> construída sobre o motor de regras (seções 2 e 3 de
> `.specs/motores-de-dominio.md`).

## O que é

Regras baseadas em condições que categorizam ou ignoram transações
automaticamente e podem ser aplicadas retroativamente às já existentes.
Também é o ponto de ligação entre uma regra e a reconciliação de um
compromisso recorrente (ver contexto de Recorrências).

## Backend

`internal/automation` — `Rule`/`Action`/`Condition`, matching via
`internal/rules` (núcleo combinável de matching, puro e sem estado —
`Condition`/`Operator`/`LogicOperator`/`Matches`), aplicação (`apply.go`)
com ações `ignore`, `set_category`, `reconcile`. A ação `reconcile` liga a
regra a um `recurrences.RecurringCommitment` sem afetar a transação
diretamente — regras reconcile-only são puladas pelo fluxo normal de
aplicação (`isReconcileOnlyRule`).

No sync, `NewTransactionHook` usa o `Querier` da transação do lançamento
para aplicar regras e o hook de desvinculação de pendências. Não abre uma
transação interna: falhas em qualquer ação revertem o lançamento, as
decisões e os eventos em conjunto. Roda em inserções e atualizações com
hash diferente; registros `unchanged` já concluídos não reaplicam regras.

`set_category` aceita qualquer categoria ativa, inclusive as de
`kind='transfer'` — é assim que se tira transferências dos totais em
massa, sem recorrer a `ignore` (ver contexto de Categorias). O seletor de
categoria da recorrência, no mesmo formulário, segue escondendo as de
transferência: um compromisso recorrente projeta receita ou despesa.

A regra `reconcile` deixou de ser a palavra final: o usuário pode
desconciliar uma ocorrência ou conciliá-la com outra transação, e essa
decisão vence a regra na resolução (`recurrences.Reconciler`). A regra
continua valendo em toda ocorrência sem decisão manual.

## Rotas HTTP

`GET/POST /api/automation-rules`,
`PUT/PATCH/DELETE /api/automation-rules/{id}`,
`GET /api/automation-rules/condition-options`.

## Frontend

`/automacoes`.
