# Especificação: Relatório Financeiro (navegação temporal, projeção e cenários)

> Não implementado ainda. Este documento materializa o pedido do usuário
> (ago/2026) por um relatório financeiro inspirado na navegação temporal dos
> relatórios do Organizze. É o doc-guarda-chuva da feature — a visão de
> produto e as decisões de arquitetura que atravessam todos os milestones.
> Cada milestone tem seu próprio spec técnico em
> `.specs/relatorio-financeiro/mN-*.md`, escrito imediatamente antes de
> implementar esse milestone. Ver também
> `.specs/plano-pagamento-e-cenarios-projecao.md` — o spec de Cenários, cuja
> entidade esta feature estende (não substitui).

## Motivação

O Contadinho hoje só olha para trás: mostra o que já aconteceu (transações
sincronizadas via Pluggy) e, no máximo, um plano de pagamento por dívida/
recebível isolado (`internal/scenarios`, ver spec de Cenários). Não existe
nenhuma tela que responda, numa única visão, perguntas como:

- Quanto terei em determinada data futura?
- Qual será meu menor saldo até lá — vou ficar sem caixa em algum momento?
- Como uma decisão hipotética (viajar, comprar um carro, trocar de emprego)
  muda essa resposta, isolada ou combinada com outras decisões?
- Minha situação está melhorando ou piorando ao longo dos meses?

O Relatório Financeiro é essa visão única. Conceitualmente:

```
Realizado → Hoje → Projeção Base → Simulação
Simulação = Projeção Base + Cenários Ativos
```

## Perguntas que o relatório precisa responder

Quanto gastei/recebi em um mês ou acumulado no ano; se meu resultado está
melhorando; onde estou gastando mais e como uma categoria evoluiu; quanto
tenho hoje; quanto terei numa data futura e qual meu menor saldo até lá; se
em algum momento fico negativo e quando; como cada cenário hipotético (e a
combinação deles) muda essas respostas; quais transações/compromissos estão
por trás de cada número apresentado (drill-down reconciliável).

## Graus de certeza

Todo valor futuro exibido carrega um tier explícito — nunca se apresenta uma
projeção como garantia:

| Tier | Significado |
|---|---|
| **Realizado** | Transação que já aconteceu (sincronizada via Pluggy). |
| **Confirmado** | Compromisso futuro explicitamente registrado (parcela de um plano de pagamento de `Payable`, ocorrência de um `RecurringCommitment`). |
| **Projetado** | Valor inferido a partir de uma regra de recorrência (ocorrência futura de um `RecurringCommitment` ainda sem transação real correspondente). |
| **Hipotético** | Transação pertencente a um `Scenario` — só existe se o cenário estiver ativo na simulação atual. |

## Lacunas de domínio que esta feature fecha

Investigação do código (ago/2026) mostrou que boa parte do que o relatório
precisa **não existe hoje**:

- **Cenários** (`internal/scenarios`) só existem como plano de pagamento de
  um `Payable` — `payable_id` é obrigatório (`CHECK (payable_id IS NOT
  NULL)`, ver `internal/db/migrations/sqlite/00013_payables.sql`). Não há
  cenário livre tipo "viagem"/"novo emprego". → resolvido no M2.
- **Não existe motor de recorrências** — salário, aluguel só aparecem depois
  de já sincronizados pela Pluggy, nunca como compromisso futuro conhecido
  hoje. → resolvido no M1.
- **Não existe serviço de saldo futuro/timeline** agregando contas — saldo é
  sempre o valor autoritativo sincronizado da Pluggy (`financial_accounts.
  balance`), nunca recalculado localmente. → resolvido no M3/M4
  (`internal/timeline`), que usa o saldo Pluggy como ponto de partida, nunca
  o recalcula.
- **Não existe biblioteca de gráficos** no frontend (dashboards são CSS
  puro/meters). → Recharts introduzido no M3.
- O motor de regras de `internal/automation` (usado hoje só para
  auto-ignorar transações) só compara texto (descrição/cartão/conta), sem
  valor/data — não serve, sem extensão, para reconciliar recorrências contra
  transações reais. → resolvido no M0 (`internal/rules`).

## Decisões de arquitetura (não reabrir sem nova conversa com o usuário)

1. **Cenários standalone**: `Scenario.PayableID` passa a aceitar nulo, com
   `Kind = "standalone"` para cenários "e se" genéricos — reaproveita a
   mesma entidade/tabelas (`Scenario`, `ScenarioTransaction`) em vez de um
   segundo conceito paralelo.
2. **Recharts** é a biblioteca de gráficos do frontend a partir desta
   feature.
3. **Fontes da projeção (v1)**: saldo atual das contas (Pluggy) + parcelas
   futuras de planos de `Payables` (`debt_plan`/`receivable_plan`, já
   existentes) + ocorrências de `RecurringCommitment` (novo) +
   `ScenarioTransaction`s de cenários `standalone` ativos. Nada além disso é
   inventado — não há "recorrência automática detectada por heurística" na
   v1.
4. **`RecurringCommitment`** é uma entidade independente, não vinculada a
   uma transação real específica — mesmo racional de isolamento já usado
   por `scenario_transactions` vs `financial_transactions` (não acoplar
   dado sincronizado externamente com anotação local de planejamento). A UI
   pode pré-preencher o formulário a partir de uma transação existente,
   como atalho, sem persistir nenhum vínculo de dados.
5. **Motor de regras compartilhado**: o núcleo combinável de matching de
   `internal/automation` (`Condition`/`Operator`/`LogicOperator`/`Matches`)
   é extraído para `internal/rules`, estendido com campos de valor/dia, e
   reaproveitado tanto por `automation` (comportamento inalterado, coberto
   por regressão) quanto pelo novo `internal/recurrences`, que o usa para
   decidir, mês a mês, se uma transação real já satisfaz a ocorrência
   esperada de um compromisso recorrente — evita duplicidade sem heurística
   fixa de "categoria+tipo+conta".

## Arquitetura de dados: a linha do tempo financeira

Peça central: `internal/timeline`, um motor que funde as 4 fontes acima
numa série única (`Series`) com um `Entry` por item de fluxo de caixa, cada
um com seu `CertaintyTier`. **Todas** as camadas de apresentação (cards,
gráfico, drill-down, comparação base×simulação) consomem essa mesma série —
nunca recalculam localmente. É o que garante a reconciliação de totais
entre cards, gráficos e detalhamento exigida pelo pedido original.

| Fonte | Tier |
|---|---|
| Transações reais (`internal/transactions`) | Realizado |
| `RecurringCommitment` — ocorrência casada com transação real no mês | Confirmado (valor real) |
| `RecurringCommitment` — ocorrência futura sem match | Confirmado (valor esperado) |
| `ScenarioTransaction` de plano `debt_plan`/`receivable_plan` não realizada | Confirmado |
| `ScenarioTransaction` de `Scenario{Kind: standalone}` ativo na simulação | Hipotético |
| Saldo atual das contas (`financial_accounts.balance`) | Realizado (ponto de partida, t=hoje) |

Detalhe técnico completo (structs, funções, endpoints) fica em cada spec de
milestone, não repetido aqui.

## Índice de milestones

Cada milestone entrega valor de forma independente e é validável
isoladamente antes do próximo. A arquitetura de dados nasce completa no M3 —
os milestones seguintes só adicionam fontes ao mesmo pipeline, nunca
retrabalham o que já existe.

- **M0** — `.specs/relatorio-financeiro/m0-motor-de-regras.md`: extração de
  `internal/rules` a partir de `internal/automation`.
- **M1** — `.specs/relatorio-financeiro/m1-recorrencias.md`: cadastro de
  compromissos recorrentes (`internal/recurrences`) — CRUD funcional sozinho.
- **M2** — `.specs/relatorio-financeiro/m2-cenarios-standalone.md`: cenários
  "e se" genéricos, sem `Payable` associado.
- **M3** — `.specs/relatorio-financeiro/m3-historico-e-situacao-atual.md`:
  `internal/timeline` básico (só Realizado) + primeira versão da tela,
  respondendo "quanto gastei/recebi" sem projeção.
- **M4** — `.specs/relatorio-financeiro/m4-projecao-base.md`: projeção
  usando payables + recorrências, menor saldo e alerta de saldo negativo.
- **M5** — `.specs/relatorio-financeiro/m5-simulacao-com-cenarios.md`:
  multi-seleção de cenários como filtro global, comparação base×simulação,
  impacto individual.
- **M6** — `.specs/relatorio-financeiro/m6-refinamentos.md`: evolução de
  categoria, comparações temporais, estados vazios, UX progressiva.

## Peças técnicas existentes reaproveitáveis

- `internal/scenarios/model.go`, `internal/scenarios/store.go`: entidade a
  estender (M2), nunca duplicar.
- `internal/automation/matching.go`: núcleo de matching a generalizar (M0).
- `internal/transactions/query.go` (`Item`, `Query`): fonte de dados reais
  que a timeline consome (M3+).
- `internal/httpapi/transactions_handlers.go`,
  `internal/httpapi/payables_handlers.go`,
  `internal/httpapi/scenarios_handlers.go`,
  `internal/httpapi/automation_handlers.go`: padrão de handler/DTO/rota a
  replicar em `timeline_handlers.go` e `recurrences_handlers.go`.
- `frontend/src/components/filters/filterUrl.ts`: mecanismo de estado na
  URL, reaproveitado para o multi-select de cenários (M5).
- `frontend/src/presentation/money.ts`: formatação BRL, valores sempre como
  string.

## Plano de implementação (execução)

O plano de implementação, com a lista de tarefas técnicas por milestone,
está em `/home/greg/.claude/plans/implemente-no-contadinho-um-cozy-quill.md`
(gerado em sessão de plan mode, ago/2026). Este spec é a referência de
produto/domínio; aquele arquivo é o checklist de execução.
