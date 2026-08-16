# M6 — Refinamentos e comparações

> Parte de `.specs/relatorio-financeiro.md`. Entrega sozinha: aprofundamento
> progressivo — evolução de categoria ao longo do tempo, comparações
> temporais, estados vazios tratados explicitamente.

## Motivação

M3-M5 entregam os números certos. M6 fecha as lacunas de UX/comparação que
o pedido original lista mas que não bloqueiam nenhuma resposta essencial —
"minha situação está melhorando?", "como uma categoria evoluiu?",
distinguir "sem dados" de "zero".

## Evolução de categoria

```go
// internal/timeline/aggregate.go
// CategoryEvolution: valores de UMA categoria mês a mês, ao longo do
// intervalo da série — para o drill-down "como evoluiu meu gasto com
// alimentação" (seção 24).
func CategoryEvolution(series Series, categoryID *string) []MonthAmount

type MonthAmount struct {
    Month  time.Time
    Amount decimal.Decimal
}
```

Reaproveita a mesma `Series` já calculada — nenhuma query nova, só um
agrupamento diferente dos `Entry`s existentes (mantém a reconciliação: o
total de `CategoryEvolution` para um mês deve bater com o item
correspondente de `CategoryBreakdown` daquele mês).

Frontend: `frontend/src/components/timeline/CategoryEvolutionChart.tsx`
(Recharts, linha por categoria), acionado ao clicar numa categoria em
`CategoryImpactList` (M3).

## Comparações temporais

```go
// internal/timeline/compare.go (mesmo arquivo do M5, adicionando funções
// que não dependem de cenários)

// MonthOverMonth compara o mês selecionado com o anterior.
func MonthOverMonth(breakdown []MonthSummary, month time.Time) (*Comparison2, bool)

// YearOverYear compara o acumulado jan..mês do ano corrente com o mesmo
// intervalo do ano anterior.
func YearOverYear(series Series, priorYearSeries Series, throughMonth time.Time) (*Comparison2, bool)

type Comparison2 struct {
    Current      decimal.Decimal
    Previous     decimal.Decimal
    DeltaPercent decimal.Decimal
}
```

`YearOverYear` precisa da série do ano anterior — o handler HTTP faz uma
segunda chamada a `BuildSeries` com `From`/`To` deslocados um ano, só
quando a comparação for solicitada (não em toda requisição de relatório).

Regra do pedido (seção 25): **não apresentar comparação quando não houver
base de dados suficiente** — `MonthOverMonth`/`YearOverYear` retornam
`ok = false` (não um valor zerado) quando o período anterior não tem
nenhuma transação, e o frontend omite o card de comparação nesse caso em
vez de mostrar "↑ ∞%" ou "↓ 100%" sem sentido.

## Estados vazios distintos de zero

Extensão de `frontend/src/components/AsyncState.tsx` (ou um componente novo
no mesmo espírito) para um terceiro estado, além de `LoadingState`/
`UnavailableState`: **"sem movimentações no período"**, usado quando
`Series.Entries` é vazio para o intervalo selecionado — visualmente
diferente de um card mostrando "R$ 0,00" (que significa "sim, teve
movimento, o resultado líquido é zero"). Aplica-se a:

- Ano sem nenhuma movimentação.
- Mês sem nenhuma movimentação (mas ano com dado).
- Cenário ativo sem nenhuma `ScenarioTransaction` dentro do período
  selecionado (mostra o cenário na lista de ativos, mas sem entrada na
  composição da projeção daquele período).

## UX progressiva

- Detalhamento (`ProjectionComposition`, drill-down de categoria) começa
  colapsado por padrão — a tela abre com a resposta simples (resumo +
  linha do tempo) e o aprofundamento é opt-in (clique para expandir),
  conforme seção 32/33 do pedido.
- Revisão final de reconciliação ponta a ponta: com o fixture completo (ver
  seção "Validação" do doc-guarda-chuva), conferir que os números de
  `SummaryCards`, `MonthlyEvolutionChart`, `CategoryImpactList` e o
  drill-down de transações batem exatamente entre si para o mesmo
  período/filtro — nenhuma camada arredonda ou recalcula de forma
  divergente das outras.

## Critério de aceite

- `go test ./internal/timeline/...`: `CategoryEvolution` reconciliável com
  `CategoryBreakdown`; `MonthOverMonth`/`YearOverYear` retornam `ok=false`
  sem dado suficiente (não um número enganoso).
- Navegação manual: clicar numa categoria e ver sua evolução mensal; ver o
  card de comparação sumir (não zerar) num período sem base de comparação;
  abrir um mês sem movimentação e ver a mensagem de "sem movimentações",
  não um card com R$ 0,00 ambíguo.
