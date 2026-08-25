# Motores de domínio (Domain Engines)

> Doc-guarda-chuva de arquitetura (não de feature): registra os motores de
> domínio reutilizáveis que o Contadinho já usa como base para múltiplas
> funcionalidades, e os princípios que os conectam. Fruto de uma conversa
> (ago/2026) sobre como documentar isso como direcionamento. Descreve o
> domínio no nível ideal, não necessariamente o estado literal do código
> hoje — onde os dois divergem, isso é dito explicitamente. Para o estado
> atual, literal, de cada feature, ver `.specs/contextos/<contexto>/
> reference.md` (ver `.specs/README.md` para o índice).

## Por que este doc existe

O Contadinho cresceu por spec de feature, e cada spec documenta bem a
própria feature. O que não estava registrado em nenhum lugar único é a
camada **abaixo** das features: um punhado de motores de domínio genéricos
que várias telas/endpoints reaproveitam. Sem esse mapa, é fácil (a)
reinventar um motor que já existe generalizado em outro pacote, ou (b)
generalizar demais algo que na verdade é específico de uma feature.

## O que conta como "domain engine"

Nem toda entidade ou pacote é um motor de domínio. O critério usado para
aceitar ou rejeitar cada candidato nesta lista:

1. **É reutilizado em múltiplos contextos** — não uma peça que só uma tela
   ou um endpoint usa.
2. **Processa/transforma, não só armazena** — um catálogo passivo
   (categorias, por exemplo) não qualifica só por ser muito referenciado;
   precisa ter comportamento (match, cálculo, fusão) que outros pilares
   reaproveitam.
3. **Não é amarrado a um provedor/implementação específica.** Pluggy é o
   provedor de dados principal hoje, mas o sistema tende a evoluir para
   lançamentos manuais também — um motor de domínio precisa fazer sentido
   independente de qual provedor está por trás.
4. **É descrito no nível ideal**, não no estado atual do código — quando os
   dois divergem, a divergência é uma nota explícita, não um fato do motor.

## Os motores

### 1. Lançamentos

*(nome provisório — ainda em aberto)*

A entidade central de fluxo de caixa e as regras que decidem o que conta
pra total: classificação de movimento, valor efetivo, inclusão/exclusão
(`internal/money`: `Classify`, `SelectEffectiveMoney`, `Considered`/
`Ignored`). Agnóstico de origem — hoje só a Pluggy popula lançamentos, mas
o motor em si não presume isso; lançamento manual é a mesma entidade, só
outra via de entrada. Ingestão (Pluggy, e no futuro entrada manual) é
provedor/entrada de dado, não parte do motor.

Nota sobre saldo: o saldo de conta reportado pelo provedor **é** a
referência para dinheiro em conta, e as regras de inclusão do motor não o
corrigem. Marcar um lançamento como ignorado é uma decisão de *relatório*
— "não conte isso nos meus totais de receita/despesa" (ex.: transferência
para uma conta não rastreada) —, não uma afirmação de que o dinheiro não
saiu do banco. O lançamento ignorado moveu dinheiro de verdade, então
continua dentro do saldo; descontá-lo reportaria um caixa que o usuário
não tem. Pelo mesmo motivo, a reconstrução histórica de patrimônio reverte
lançamentos ignorados ao caminhar para trás (`internal/networth`
`cashDeltasDescending`).

Onde as regras de inclusão *decidem* o número é em outro lugar: nos totais
de receita/despesa, e na dívida de cartão — ali a pergunta é "quanto esta
fatura vai me cobrar", que uma decisão de inclusão legitimamente molda, e
por isso o total sai dos lançamentos elegíveis do ciclo e não do saldo
reportado (`transactions.CreditCardTransactionTotal`). O caixa em conta
vem de `transactions.CashOnHand`, uma implementação só, compartilhada pela
Timeline (âncora de hoje) e pelo Patrimônio Líquido.

### 2. Motor de regras

**Pacote:** `internal/rules`

Núcleo de matching combinável (`Condition`/`Operator`/`LogicOperator`/
`Matches`), puro e sem estado — testa um `MatchCandidate` contra condições
de campo/operador/valor com lógica AND/OR. Consumido hoje por Automação
(matching de regras aplicadas a lançamentos) e, indiretamente, por
Recorrências via Automação (ver seção "Não são motores" abaixo).

### 3. Automação

**Pacote:** `internal/automation`

Liga o resultado de uma regra a um efeito sobre lançamentos reais: uma
`Rule` com condições (via Motor de regras) e uma ou mais ações (`ignore`,
`set_category`, `reconcile`), aplicável a lançamentos novos ou
retroativamente.

### 4. Payables

**Pacote:** `internal/payables`

Rastreia um **alvo real**: um total que existe (dívida, recebível, ou meta)
e vai sendo abatido/atingido por lançamentos reais vinculados, com
settled/remaining/status sempre recomputados na leitura, nunca persistidos.

Escopo generalizado além do estado atual do código: hoje o `Kind` só cobre
`debt`/`receivable`; a direção é generalizar para incluir metas (ex.:
"juntar R$10.000 até dezembro") sob o mesmo motor — mesma mecânica de
total + vínculo real + progresso recomputado, sem dívida real por trás.
Isso exige estender `Kind` e a lógica de elegibilidade de vínculo
(`eligibility.go`), que hoje deriva a direção de fluxo aceita a partir do
`Kind` debt/receivable.

Nota de nome: "Payables" deixa de descrever bem o motor quando inclui
metas (nem toda meta é um "a pagar"). Mantido por ora; nomes alternativos
("Plano de pagamentos", "Compromisso", "Alvo financeiro") ficam para
avaliação num refactor futuro.

### 5. Cenários

**Pacote:** `internal/scenarios`

A **camada hipotética**: projeções autoradas pelo usuário, nunca por fato
sincronizado. Duas formas, mesma entidade (`Scenario`/`ScenarioTransaction`):

- **Presa a um `Payable`**: plano de parcelas de um `Payable` específico —
  `debt_plan`/`receivable_plan` hoje, e (seguindo a generalização do motor
  4) um futuro `goal_plan` seria só mais um `Kind` nessa mesma família, sem
  mudança estrutural em `scenarios`.
- **Livre** (`standalone`): cenário "e se" sem `Payable` nenhum — "viagem",
  "novo emprego".

Princípio central: **nunca contamina totais reais por padrão.** Um cenário
nunca é "ativado" como estado persistido — ele entra num relatório apenas
quando o chamador passa `scenario_id` explicitamente, ou é incluído numa
seleção de simulação — a "ativação" é um parâmetro de consulta, não uma
flag gravada.

### 6. Timeline

**Pacote:** `internal/timeline`

O motor de fusão: junta lançamentos reais e o fluxo unificado de projeção de
cenários numa única `Series`, com um `CertaintyTier` explícito por `Entry`
(Realizado / Confirmado / Projetado / Hipotético). Toda camada de
apresentação (cards, gráfico, drill-down) lê dessa mesma `Series` — nunca
recalcula localmente.

Fontes hoje: Lançamentos + Cenários (**2**). A nota de direção futura que
esta seção trazia se concretizou: recorrência deixou de ser entidade
paralela com motor de ocorrência próprio e virou `Scenario{Kind:
recurring}`, então `BuildSeries` fala com uma fonte de planejamento só
(`projections.List`). O motor **não** conhece Payables diretamente — o
vínculo com um `Payable` fica encapsulado dentro de Cenários
(`Scenario.PayableID`/`Kind`), e é isso que decide se uma
`ScenarioTransaction` vira tier Confirmado (presa a um Payable) ou
Hipotético (`standalone`).

**Nota de divergência:** o `CertaintyTier` é produzido e transportado até a
API, mas nenhuma tela o exibe hoje — ver
`.specs/contextos/relatorio-financeiro/reference.md`, seção Notas. O
princípio 4 abaixo continua valendo como régua de domínio; o que falta é a
expressão dele na interface.

## Não são motores (features/entidades que compõem os motores acima)

- **Recorrências** (`internal/recurrences`): feature, não motor —
  compromisso de fluxo de caixa conhecido de antemão (salário, aluguel),
  reconciliado contra lançamentos reais compondo Motor de regras +
  Automação (uma `Rule` com ação `reconcile` aponta pro compromisso; sem
  essa regra e sem decisão manual, ele nunca reconcilia). A decisão manual
  do usuário vence a regra — ver `.specs/contextos/recorrencias/
  reference.md`, seção "Conciliação manual". A concepção original —
  recorrências construídas a partir de Cenários — é o que o código faz
  hoje: um compromisso recorrente **é** um `Scenario{Kind: recurring}` mais
  sua linha em `scenario_recurring_schedules`, e o id do cenário é a
  identidade que projeção, alvo de automação e decisões de ocorrência
  compartilham. O pacote `recurrences` continua existindo como o motor de
  ocorrência e conciliação (calendário, `Reconciler`), não mais como uma
  entidade paralela com tabela própria.
- **Categorização** (`internal/categories`): catálogo de categorias +
  histórico. Muito referenciado, mas é um catálogo passivo, sem lógica de
  match/cálculo/fusão própria — não qualifica pelo critério 2.
- **Net worth** (`internal/networth`): snapshots/histórico de patrimônio.
  Não qualifica como motor — não investigado a fundo, mas não atende ao
  critério 3 (não reutilizado em múltiplos contextos hoje).
- **Sincronização/Pluggy** (`internal/pluggy`, `internal/syncsvc`,
  `internal/worker`): provedor de ingestão, não motor de domínio — viola o
  critério 3 diretamente. Alimenta o motor de Lançamentos, mas o sistema
  tende a ganhar outras vias de entrada (lançamento manual) que não têm
  nada a ver com Pluggy.

## Princípios transversais (valem para todo motor novo)

1. **Recomputar na leitura, nunca persistir valor derivado.**
   `Payable.SettledAmount/RemainingAmount/Status`,
   `ScenarioTransaction.Status` — todos calculados a partir dos dados-fonte
   a cada leitura.
2. **Isolar dado externo de anotação local.** Nenhuma entidade de
   planejamento (`ScenarioTransaction`, `RecurringCommitment`) tem FK dura
   para lançamentos reais. O vínculo, quando existe, é uma tabela de
   ligação explícita (`payable_transaction_links` e `scenario_realizations`,
   esta última cobrindo as três formas — `settlement`, `allocation`,
   `reconciliation`) — nunca o lançamento real "sabe" que está sendo usado
   para planejamento. A linha `reconciliation` é o caso-limite que mostra o
   princípio 1 junto: o que ela grava é a *decisão* do usuário sobre uma
   ocorrência (conciliar com X, ou desconciliar), não o estado de
   conciliação — esse segue recomputado a cada leitura, consultando a
   decisão antes da regra de automação.
3. **Hipotético é opt-in explícito, nunca ambiente global.** Cenário e
   projeção só entram num relatório quando o parâmetro pede — nunca uma
   flag "modo simulação" ligada globalmente.
4. **Vocabulário de certeza compartilhado.** Os 4 tiers da Timeline
   (Realizado/Confirmado/Projetado/Hipotético) são a régua comum — qualquer
   motor novo que produza "algo que ainda vai acontecer" deveria se
   encaixar num desses tiers em vez de inventar um novo grau de certeza.
5. **Um dia é um dia só.** Todo motor que raciocina em dias de calendário
   (entries da Timeline, parcelas de cenário, vencimento de fatura) trunca
   por `internal/dates.Day` — meia-noite UTC, a partir do relógio de parede
   de quem chama. Cada motor tinha crescido a própria cópia; uma definição
   compartilhada é o que faz dois motores que não podem depender um do
   outro compararem o mesmo dia com `==`. A exceção deliberada é a
   matemática de ciclo de fatura (`transactions/cardtotal.go`), que fecha
   em `time.Local` de propósito — outra convenção, não uma cópia.

   `internal/dates.DaysInMonth` segue a mesma regra para a outra pergunta
   de calendário que os motores compartilham: quantos dias tem um mês, para
   encaixar uma cadência mensal num mês mais curto (fechamento no dia 31
   cai no dia 30 em novembro, não no dia 1º de dezembro). Não recebe
   `*time.Location` de propósito — a contagem é aritmética de calendário,
   não um instante, então nenhum fuso a muda; as três cópias que ela
   substituiu discordavam só na rota, nunca no número.

## Notas abertas (para avaliação futura, não decisões)

1. Nome definitivo para o motor de Lançamentos.
2. Nome definitivo para Payables generalizado (dívida/recebível/meta).
3. ~~Se/quando Recorrências passa a se construir a partir de Cenários —
   impacto direto: Timeline colapsa de 3 para 2 fontes.~~ **Resolvido:**
   feito, e a Timeline colapsou. Ver seção Timeline.
4. Se Payables generalizado exige `Kind` fechado crescendo (`goal` como
   mais um valor) ou um modelo de elegibilidade de vínculo mais aberto —
   não decidido nesta conversa.
5. Sinal da parcela de plano: `scenarios.SignedAmount` **nega** o valor de
   um `debt_plan` em vez de forçar a direção do fluxo, e nada hoje rejeita
   uma parcela gravada com `Amount` negativo — que então leria como
   entrada num plano de dívida. Comportamento preservado do código
   anterior; decidir entre validar na escrita ou forçar o sinal na leitura
   muda totais reais, então fica registrado aqui e não resolvido de
   passagem num refactor.
