# Recorrências

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature. Não é um motor de domínio — feature
> construída sobre Motor de regras + Automação (ver "Não são motores" em
> `.specs/motores-de-dominio.md`, que também nota a divergência de design
> descrita abaixo).

## O que é

Compromisso de fluxo de caixa conhecido de antemão (salário, aluguel),
cadastrado manualmente, independente de qualquer transação real
específica, reconciliado contra transações reais via regra de automação.

## Estado vs. ideal

A concepção original (`.specs/motores-de-dominio.md`, notas abertas) era
Recorrências serem construídas a partir de Cenários. O código foi
implementado de forma diferente: campos de agendamento próprios
(`Cadence`, `DayOfMonth`/`MonthOfYear`), sem relação com `internal/scenarios`.
Se isso mudar num refactor futuro, a Timeline colapsaria de 3 para 2
fontes — ver contexto de Relatório Financeiro.

## Backend

Na Timeline, uma ocorrência só vira entry se **não** foi reconciliada, e
com tier **Projetado**; a reconciliada não gera entry porque a transação
real já a carrega. Os candidatos que uma ocorrência tenta reconciliar são
escopados ao mês dela — sem isso, a transação de um mês reconciliava as
ocorrências de todos os outros.

`internal/recurrences` — `RecurringCommitment` (`Kind` income/expense,
`Amount`, `CategoryID`, `AccountID`, `Cadence` monthly/annual,
`DayOfMonth`/`MonthOfYear`, `StartDate`/`EndDate`, `IsActive`). Sem
critério de matching próprio: ocorrências vêm só do agendamento
(`OccurrencesInRange`); reconciliação contra transações reais acontece na
leitura via `internal/rules` (`Matches`/`MatchCandidate`), dirigida por
regras de `internal/automation` cuja única ação é `reconcile`
(`ListActiveReconcileTargets` — sem essa regra, o compromisso nunca
reconcilia).

## Rotas HTTP

`GET/POST /api/recurring-commitments`,
`PUT/PATCH/DELETE /api/recurring-commitments/{id}`.

## Frontend

`/recorrencias`.
