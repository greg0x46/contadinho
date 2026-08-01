# Especificação: Plano de pagamento de dívidas + cenários de projeção financeira

> Não implementado ainda. Este documento materializa uma discussão de design
> entre o usuário e o assistente (ago/2026) sobre como a tela de Dívidas
> ganharia um "plano de pagamento" e, de forma mais geral, como o app poderia
> representar transações hipotéticas/futuras que entram opcionalmente em
> relatórios. Ver também `.backlog/teto-gasto-diario.md` — outra feature que
> segue o mesmo princípio de "recomputar tudo na leitura, nunca persistir
> valor derivado".

## Motivação

Hoje "Dívidas" (`internal/debts/`) é puramente retrospectivo: uma dívida é
quitada vinculando transações bancárias que **já aconteceram**
(`debt_transaction_links`, FK dura pra `financial_transactions`). Não existe
campo, tabela ou tela que responda "como eu pretendo pagar isso" — só "o que
eu já paguei".

O pedido tem duas camadas:
1. Dar à tela de dívida um "plano de pagamento" que olhe pra frente.
2. Fazer isso com uma entidade **genérica o suficiente** para outras
   features futuras de projeção de cenário ("e se eu financiar um carro de
   R$800/mês", por exemplo) — não uma tabela dívida-específica que precisaria
   ser reinventada depois.

## Por que não reaproveitar `financial_transactions`

Já existe um status `PENDING` nessa tabela (parcela futura de cartão que o
Open Finance já confirmou) que **entra automaticamente** em totais e
relatórios (`internal/money/money.go`, `eligibleProviderStatuses`). Isso
funciona porque é um fato real, só que ainda não debitado.

Uma projeção de cenário é diferente: é **hipotética**, inventada pela
pessoa. Se morasse na mesma tabela, todo relatório que soma totais
precisaria lembrar explicitamente de excluir essas linhas — fácil vazar
número hipotético pra dentro de um total real (ex.: "valor restante" da
dívida passando a incluir uma parcela que na verdade ainda não existe).
Por isso a proposta usa tabelas novas e aditivas: nada do modelo atual muda
de comportamento sem pedir explicitamente.

## Non-goals desta v1

- Não reformula `financial_transactions` nem `debt_transaction_links` — os
  dois continuam exatamente como são.
- Sem matching automático de "essa transação real quita qual parcela
  projetada" — a pessoa escolhe manualmente na v1; sugestão automática por
  proximidade de valor/data fica pra v2.
- `scenarios.kind` só implementa `'debt_plan'` nesta v1. `'what_if'`
  (cenário livre, não amarrado a uma dívida) é citado como extensão futura
  possível, não é construído agora.

## Modelo de dados

```sql
CREATE TABLE scenarios (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind       TEXT NOT NULL,              -- 'debt_plan' nesta v1
    name       TEXT NOT NULL,
    debt_id    UUID REFERENCES debts(id) ON DELETE CASCADE,  -- obrigatório quando kind = 'debt_plan'
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE scenario_transactions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scenario_id  UUID NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
    description  TEXT NOT NULL,
    amount       NUMERIC NOT NULL,   -- valor planejado; mesma convenção de sinal de financial_transactions.amount
    projected_at DATE NOT NULL,      -- data prevista
    category     TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Aloca (parcial ou totalmente) um vínculo real de transação-com-dívida a
-- uma parcela planejada. Many-to-many: um link pode cobrir mais de uma
-- parcela (pagamento em lote) e uma parcela pode ser coberta por mais de um
-- link (pagamento fracionado em mais de uma transação real).
CREATE TABLE scenario_transaction_realizations (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scenario_transaction_id  UUID NOT NULL REFERENCES scenario_transactions(id) ON DELETE CASCADE,
    debt_link_id             UUID NOT NULL REFERENCES debt_transaction_links(id) ON DELETE CASCADE,
    allocated_amount         NUMERIC NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Structs Go (`internal/scenarios/model.go`, espelhando o estilo de
`internal/debts/model.go`):

```go
type Scenario struct {
    ID        string
    Kind      string // "debt_plan"
    Name      string
    DebtID    *string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type ScenarioTransaction struct {
    ID          string
    ScenarioID  string
    Description string
    Amount      decimal.Decimal // valor planejado
    ProjectedAt time.Time
    Category    *string
}

type ScenarioTransactionRealization struct {
    ID                    string
    ScenarioTransactionID string
    DebtLinkID            string
    AllocatedAmount       decimal.Decimal
}
```

### Status: recomputado na leitura, nunca gravado

Igual ao padrão de `Debt.PaidAmount`/`RemainingAmount`/`StatusFor`
(`internal/debts/model.go`) — nenhum status é persistido. `realizedTotal` é
`SUM(allocated_amount)` das `scenario_transaction_realizations` daquela
parcela, calculado na query, não guardado:

```go
func (s ScenarioTransaction) Status(today time.Time, realizedTotal decimal.Decimal) string {
    switch {
    case realizedTotal.IsZero() && today.After(s.ProjectedAt):
        return "atrasada"
    case realizedTotal.IsZero():
        return "projetada"
    case realizedTotal.LessThan(s.Amount):
        return "paga_parcialmente"
    case realizedTotal.Equal(s.Amount):
        return "paga"
    default: // realizedTotal > s.Amount
        return "paga_a_mais"
    }
}
```

## Fluxo: plano de pagamento de uma dívida

1. Ao criar um plano numa dívida, o app cria um `Scenario{kind: "debt_plan",
   debt_id: <dívida>}` e um conjunto de `scenario_transactions` (geradas
   automaticamente a partir de valor restante ÷ nº de meses, editáveis
   depois, ou cadastradas à mão).
2. Quando a pessoa vincula uma transação real à dívida (fluxo que já existe,
   `debt_transaction_links`), a UI oferece "isso quita qual parcela
   planejada?" — ao confirmar, cria uma ou mais
   `scenario_transaction_realizations` alocando (parte d)o valor do link
   à(s) parcela(s) escolhida(s). Um único link pode ser dividido entre
   parcelas (pagamento em lote cobrindo 2 meses de uma vez); uma única
   parcela pode receber alocações de mais de um link (pagamento fracionado
   em duas transações reais).
3. `Debt.PaidAmount`/`RemainingAmount` continuam calculados **só** a partir
   de `debt_transaction_links` como hoje — o plano nunca influencia o valor
   real da dívida, é somente uma camada de acompanhamento por cima.

## Divergência entre planejado e realizado

Pagar um valor diferente do previsto numa parcela é o caso normal, não
exceção — a query de `realizedTotal` (seção anterior) já responde "pagou
menos", "pagou certo" ou "pagou mais" por parcela. O que fica **decidido**
sobre o que fazer com isso:

- **Nada é recalculado automaticamente.** O plano é uma previsão editável
  pela pessoa, não um contrato que se reajusta sozinho — pagar R$300 a menos
  este mês não permite ao sistema adivinhar se a intenção é "compenso mês
  que vem", "vou renegociar" ou só um desvio pontual. As parcelas futuras
  permanecem como estão até alguém mexer nelas.
- **O desvio acumulado é exposto, não escondido.** `Σ amount − Σ
  realizedTotal` de todas as parcelas com `projected_at <= hoje` dá o mesmo
  tipo de leitura que a proposta A do mockup ("ritmo necessário vs. seu
  ritmo real"), só que derivado das parcelas em vez de uma meta solta — as
  duas propostas de UI não são mutuamente exclusivas, B pode mostrar o
  insight de A por cima.
- **Reagir ao desvio é uma ação manual e explícita**: "Reajustar parcelas
  restantes" regera as parcelas futuras a partir do saldo ainda não
  realizado. Nunca dispara sozinho — só quando a pessoa clica.

### Reajustar parcelas restantes: quais parcelas entram, quais ficam de fora

O reajuste **só mexe em parcelas com `realizedTotal = 0`** (nada alocado
ainda — inclui tanto `projetada` quanto `atrasada` sem nenhum pagamento).
Uma parcela com qualquer alocação, mesmo parcial (`paga_parcialmente`,
`paga`, `paga_a_mais`), nunca é apagada nem recalculada pelo reajuste — ela
representa uma alocação real que já aconteceu e fica registrada como está.

```
saldo_a_redistribuir = Debt.RemainingAmount
                        − Σ (amount − realizedTotal) das parcelas
                          com realização parcial preservadas
```

(ou seja: pega o que falta pagar de verdade na dívida, e desconta a fatia
que já está "reservada" pelo restinho em aberto de parcelas parcialmente
pagas — o que sobra é o que as parcelas *novas* precisam somar.)

As parcelas afetadas (sem nenhuma alocação) são substituídas por um novo
conjunto, gerado por uma de duas estratégias que a pessoa escolhe no
momento do reajuste:

**1. Abater do final** — mantém o valor da parcela, muda o prazo.
```
valor_parcela     = valor de referência (ex.: amount da última parcela do plano anterior)
nº_novas_parcelas = ceil(saldo_a_redistribuir / valor_parcela)
```
Pagou a mais → menos parcelas → quita antes do previsto.
Pagou a menos → mais parcelas → quitação prevista empurra pra frente.
A última parcela absorve o resto da divisão pra bater o total exato.

**2. Redistribuir entre os meses** — mantém o prazo, muda o valor.
```
nº_parcelas   = quantidade de parcelas futuras sem alocação que existiam
                antes do reajuste (preserva a data de quitação original)
valor_parcela = saldo_a_redistribuir / nº_parcelas
```
A quitação prevista não muda; o valor mensal sobe ou desce pra compensar.
A última parcela absorve o resto da divisão.

Nos dois casos: as parcelas antigas afetadas são apagadas e substituídas —
esta v1 não mantém histórico de "parcela substituída por reajuste de
DD/MM" (ver pergunta em aberto sobre isso).

## Relatórios: opt-in explícito, nunca automático

Nenhum endpoint existente passa a incluir projeção por padrão. Um endpoint
como `spending-by-category` (`internal/transactions/query.go`) ganharia um
parâmetro opcional (`scenario_id=...`) que faz um union das
`scenario_transactions` daquele cenário — convertidas pro mesmo formato de
`Item` usado hoje, mas com um campo `source: "real" | "projetado"` explícito
na resposta, para a UI nunca confundir os dois.

## UI

Ver mockup das 3 direções discutidas (artifact
`plano-pagamento-mockups.html`, propostas A/B/C — meta+projeção, parcelas
planejadas, linha do tempo). As propostas B e C são as que precisam de
compromissos futuros explícitos — ambas passam a ser, na prática, uma tela
que lê/escreve `scenario_transactions` do `Scenario` `debt_plan` da dívida.

A tag "Pendente" das parcelas planejadas deve ficar visualmente diferente de
"Paga"/"Atrasada" (que são fatos reais) — ex.: tag tracejada "Projeção" —
para deixar claro que aquele valor ainda não aconteceu.

## Peças técnicas existentes reaproveitáveis

- `internal/debts/model.go`, `internal/debts/store.go`: padrão de
  recomputar tudo na leitura, nunca persistir valor derivado — espelhar
  exatamente para `ScenarioTransaction.Status`.
- `internal/money/money.go`: `SelectEffectiveMoney`, `CanonicalDecimal` —
  reusar para normalizar `scenario_transactions.amount` no mesmo formato
  usado pelos relatórios de transações reais quando unidos numa resposta.
- `internal/transactions/query.go` (`Item`, `fetchAllViews`): forma alvo pra
  onde `scenario_transactions` precisa ser convertida ao entrar num
  relatório via opt-in.

## Perguntas em aberto / próximos passos

1. Qual das propostas de UI (A/B/C) vira a primeira tela — proposta A
   (meta+projeção) é o menor passo, mas não materializa
   `scenario_transactions` de forma visível; B ou C são as que realmente
   testam esta entidade em produção.
2. Desenho exato do endpoint de criação/edição de plano (uma request que
   cria `Scenario` + N `scenario_transactions` de uma vez vs. endpoints
   separados).
3. Se/quando vale a pena investir em sugestão automática de matching
   (transação real → parcela projetada mais próxima por valor e data) — a
   alocação em si (`scenario_transaction_realizations`) continua manual
   mesmo se isso for implementado; a sugestão só pré-preenche a escolha.
4. Se `kind='what_if'` (cenário solto, sem dívida) chega a ser necessário
   logo — hoje é só uma porta deixada aberta no `kind`, sem UI nem caso de
   uso concreto ainda.
5. Se o reajuste manual (seção "Reajustar parcelas restantes") entra na v1
   ou fica pra depois — sem ele, quem quiser reagir a um desvio precisa
   editar as parcelas futuras uma a uma à mão. Ver roadmap abaixo (task 7).
6. Se vale manter histórico de "essa parcela foi substituída num reajuste
   de DD/MM" ou se é aceitável simplesmente apagar as parcelas antigas sem
   rastro — a v1 do reajuste assume que apagar é suficiente.

## Roadmap de entrega

Cada tarefa entrega algo que funciona e é verificável sozinho, sem depender
das tarefas seguintes já existirem. A ordem é a ordem de dependência.

1. **Fundação: `Scenario` + `ScenarioTransaction` (CRUD, sem realização)**
   Migração das duas primeiras tabelas + CRUD backend (criar cenário
   `debt_plan` pra uma dívida; criar/editar/excluir parcelas uma a uma).
   *Valor:* já dá pra gravar e consultar um plano por API, mesmo sem UI.
   *Validação:* testes de integração do store + chamadas manuais via
   `curl`/Postman criando um cenário e algumas parcelas.

2. **Geração automática de parcelas**
   Endpoint que recebe `Debt.RemainingAmount` + nº de meses (ou valor de
   parcela) e cria N `scenario_transactions` mensais.
   *Valor:* poupa a pessoa de cadastrar parcela por parcela à mão.
   *Validação:* teste de unidade garantindo que a soma das parcelas geradas
   bate com o valor restante (com a última absorvendo o resto da divisão).

3. **Tela do plano de pagamento (proposta B) — leitura + geração**
   UI na `DebtDetailPage`: tabela de parcelas + botão "Gerar parcelas".
   Sem edição de status ainda (isso é a task 5).
   *Valor:* primeira coisa realmente usável de ponta a ponta — a pessoa cria
   e vê um plano de pagamento na tela.
   *Validação:* manual no browser — criar plano numa dívida de teste, ver a
   tabela renderizar com os valores certos.

4. **Status por data (`projetada` / `atrasada`), sem realização**
   `ScenarioTransaction.Status` só com a checagem de data (sem
   `realizedTotal`, que ainda não existe).
   *Valor:* a tabela já sinaliza visualmente parcela vencida sem paga.
   *Validação:* parcela com `projected_at` no passado aparece com a tag
   "Atrasada" na tela criada na task 3.

5. **Realização: alocação de vínculo real a parcela planejada**
   Tabela `scenario_transaction_realizations` + fluxo "isso quita qual
   parcela?" no momento de vincular uma transação real à dívida (UI
   existente de "Vincular transação"). `Status` passa a usar
   `realizedTotal` (os 5 estados completos).
   *Valor:* o plano deixa de ser só uma lista estática — reflete pagamento
   real, inclusive parcial/a mais/em lote.
   *Validação:* vincular uma transação real, alocar a uma parcela, ver o
   status mudar pra `paga`/`paga_parcialmente`/`paga_a_mais` conforme o
   valor alocado.

6. **Desvio acumulado exibido na tela**
   Cálculo e exibição de `Σ amount − Σ realizedTotal` das parcelas vencidas.
   *Valor:* responde "estou atrasado ou adiantado no plano?" sem precisar
   somar linha por linha.
   *Validação:* montar um cenário de teste com uma parcela paga a menos e
   conferir que o desvio exibido bate com a conta manual.

7. **Reajustar parcelas restantes (abater do final / redistribuir entre os
   meses)**
   Ação manual descrita acima, com as duas estratégias selecionáveis.
   *Valor:* fecha o loop "desviei do plano → decido como replanejar" sem
   precisar editar parcela por parcela à mão.
   *Validação:* gerar desvio de propósito (pagar mais ou menos que o
   planejado), acionar cada estratégia separadamente, e conferir que o novo
   conjunto de parcelas soma exatamente `saldo_a_redistribuir` — e que
   parcelas com alocação prévia não foram tocadas.

8. **Relatórios: opt-in de cenário em `spending-by-category`**
   Parâmetro `scenario_id` unindo `scenario_transactions` na resposta, com
   `source: "real" | "projetado"` explícito.
   *Valor:* primeiro uso da entidade fora da tela de dívida — projeção
   aparecendo (sob pedido) num relatório existente.
   *Validação:* chamar o endpoint com e sem `scenario_id` e confirmar que o
   total muda só quando o parâmetro é passado.

Tasks 1–4 formam o corte mínimo pra ter algo demonstrável (plano estático,
sem realização). Tasks 5–7 são o que realmente responde à pergunta original
("pagamento diferente do planejado"). Task 8 é independente das 5–7 e pode
ser feita em paralelo a partir da task 1.
