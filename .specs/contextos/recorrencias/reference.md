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
(`ListActiveReconcileTargets` — sem essa regra e sem decisão manual, o
compromisso nunca reconcilia).

### Conciliação manual

Sobre essa resolução automática existe uma camada de **decisão do
usuário**, e só ela é persistida (`recurrence_reconciliations`, uma linha
por ocorrência, `UNIQUE (recurring_commitment_id, occurrence_date)` e
`UNIQUE (transaction_id)`). O estado resolvido continua recomputado a cada
leitura — princípio 1 de `.specs/motores-de-dominio.md` —, e a tabela é a
tabela de ligação explícita que o princípio 2 pede.

`Reconciler` (`reconciliation.go`) é o único lugar que resolve uma
ocorrência, usado tanto pela Timeline quanto pela camada HTTP. Ordem:

1. override `detached` → não conciliada, e a regra **não** pode reclamá-la
   de volta. Isso é o que faz "desconciliar" funcionar: a regra é
   reavaliada do zero a cada leitura, então apagar um registro não bastaria;
2. override `linked` → a transação escolhida à mão (`OriginManual`);
3. senão, a regra de automação, escopada ao mês da ocorrência
   (`CandidatesByMonth`, movida de `internal/timeline`) — `OriginRule`.

No passo 3 as transações já vinculadas à mão saem do pool de candidatas: sem
isso, uma transação conciliada em março seria também casada pela regra em
abril e o mesmo dinheiro suprimiria duas projeções.

Elegibilidade de vínculo manual em `eligibility.go`, espelhando
`internal/payables`: não ignorada, BRL, direção compatível com o `Kind`, e
não conciliada em outra ocorrência. Diverge de `CandidatesByMonth` de
propósito ao **não** exigir a categoria/conta do compromisso — conciliar à
mão é justamente o que se faz quando a categoria da transação está errada.

Uma transação marcada como ignorada perde o vínculo (`UnlinkIfPresent`, no
`onIgnoredHook` junto com `payables`); uma ocorrência desconciliada não é
afetada por isso, porque a decisão é sobre a ocorrência.

## Rotas HTTP

`GET/POST /api/recurring-commitments`,
`PUT/PATCH/DELETE /api/recurring-commitments/{id}`.

Conciliação — o endereço de uma ocorrência é o compromisso mais o dia, já
que ocorrência não tem id armazenado:
`GET /api/recurring-commitments/{id}/occurrences`,
`GET .../occurrences/{date}/candidates`,
`PUT .../occurrences/{date}/reconciliation` (`{"state":"linked",
"transaction_id":…}` ou `{"state":"detached"}`),
`DELETE .../occurrences/{date}/reconciliation` (volta ao automático).
Do lado da transação, só leitura:
`GET /api/transactions/{id}/reconciliation` — as escritas passam pelos
endpoints acima, para existir um único lugar que decide o que é uma
conciliação válida.

## Frontend

`/recorrencias` — cada compromisso expande mostrando suas ocorrências do
período, com a transação que as conciliou e as ações Conciliar com… /
Desconciliar / Voltar ao automático.
`/transacoes` — seção "Conciliação" no drawer de detalhe.
