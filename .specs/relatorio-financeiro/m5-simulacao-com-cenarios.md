# M5 — Simulação com Cenários (multi-seleção)

> Parte de `.specs/relatorio-financeiro.md`. Entrega sozinha: responde "como
> ficarei se eu viajar e comprar um carro", com impacto individual e
> combinado de cada decisão hipotética.

## Motivação

M4 entrega a projeção base (o que já é conhecido). M5 adiciona a camada de
simulação: `Simulação = Projeção Base + Cenários Ativos`, onde os cenários
ativos são um subconjunto escolhido pelo usuário dentre os
`Scenario{Kind: standalone}` (M2) — cumulativo, com impacto individual
visível por cenário, sem exigir sair do relatório para ativar/desativar
(seções 14–21 do pedido).

## Extensão de `internal/timeline/build.go`

Terceiro passo de fonte, condicional a `BuildParams.ScenarioIDs`:

```go
// scenarioEntries só roda para os Scenario{Kind: standalone} cujos IDs
// estão em ScenarioIDs — nunca todos os cenários existentes (regra
// fundamental do domínio: cenários inativos nunca aparecem, seção 30).
// Tier: Hipotético.
func scenarioEntries(ctx context.Context, q Querier, scenarioIDs []string, from, to time.Time) ([]Entry, error)
```

## `internal/timeline/compare.go` (novo arquivo)

```go
// CompareBaseVsSimulation roda BuildSeries duas vezes com os mesmos
// filtros (uma com ScenarioIDs vazio, outra com os ativos) e compara os
// dois resultados ponto a ponto — saldo final, menor saldo, resultado do
// período.
func CompareBaseVsSimulation(base, simulation Series) Comparison

type Comparison struct {
    BaseFinalBalance       decimal.Decimal
    SimulationFinalBalance decimal.Decimal
    Impact                 decimal.Decimal // Simulation - Base
    BaseLowestBalance       DayPoint
    SimulationLowestBalance DayPoint
}

// ScenarioImpact isola o efeito de UM cenário: roda BuildSeries com
// [scenarioID] sozinho (ou subtrai simulação-completa menos
// simulação-sem-esse-cenário) e retorna o delta no saldo final do período
// analisado. Seção 19: "apresentar apenas o impacto dentro do período
// atualmente analisado".
func ScenarioImpact(ctx context.Context, q Querier, base BuildParams, scenarioID string) (Impact, error)

type Impact struct {
    ScenarioID   string
    ScenarioName string
    Delta        decimal.Decimal
}
```

Custo: com N cenários ativos, calcular o impacto individual de cada um
custa N chamadas adicionais a `BuildSeries` (uma por cenário isolado) além
da chamada base e da chamada com todos ativos — aceitável para o número de
cenários que um usuário realisticamente ativa ao mesmo tempo (poucas
unidades), não otimizado para dezenas.

## Endpoint HTTP

`GET /api/timeline?...&scenario_ids=uuid1,uuid2` — quando `scenario_ids` não
é vazio, o handler chama `BuildSeries` para Base (`scenario_ids=[]`) e para
Simulação (`scenario_ids=params`), mais `ScenarioImpact` para cada id, e
retorna tudo num payload só:

```json
{
  "base": { "...": "Series" },
  "simulation": { "...": "Series" },
  "scenario_impacts": [{ "scenario_id": "...", "scenario_name": "...", "delta": "-8000.00" }]
}
```

Economiza idas e vindas do frontend — uma requisição já traz o comparativo
completo.

## Frontend

- `frontend/src/components/timeline/ScenarioMultiSelect.tsx` — `Select
  mode="multiple"` sobre `useScenarios({ kind: "standalone" })` (M2).
  Estado sincronizado com a URL via `useSearchParams` +
  `frontend/src/components/filters/filterUrl.ts` (mesmo mecanismo já usado
  pelos filtros existentes — `scenario_ids` vira mais um campo do objeto de
  filtros da página, serializado como CSV). Adicionar/remover um cenário
  muda a URL, o que muda a `queryKey` de `useTimeline`, o que já dispara o
  refetch via React Query — nenhuma lógica extra de recálculo precisa ser
  escrita (seção 17: "recalcula tudo imediatamente").
- `frontend/src/components/timeline/BaseVsSimulationCompare.tsx` — mostra
  lado a lado saldo projetado base × com cenários × impacto (seção 18),
  usando `Comparison` do endpoint.
- `frontend/src/components/timeline/ProjectionComposition.tsx` — breakdown
  de entradas/saídas/cenários com `scenario_impacts[]` (seção 19), cada
  cenário com seu delta individual, drill-down por `Entry.Source`.
- `ProjectionTimeline.tsx` (M4) ganha uma segunda linha (Simulação) quando
  há cenários ativos — sempre **Base vs Simulação**, nunca uma linha por
  cenário individual mesmo com vários ativos (seção 20: evitar rabisco de
  N linhas).
- Chips de cenário ativo removíveis individualmente (seção 15/16):
  `[ Viagem × ] [ Comprar carro × ] [+ Adicionar cenário]`, com ação
  "remover todos" voltando à projeção base.

## Regra fundamental (seção 30, reforçada aqui)

Transações hipotéticas de cenários inativos nunca entram em `scenarioEntries`
— o filtro é estritamente `ScenarioIDs` recebido, nunca "todos os cenários
existentes". Selecionar/remover um cenário afeta só a chamada corrente à
API, nunca dados reais nem outros relatórios.

## Critério de aceite

- `go test ./internal/timeline/...`: `compare_test.go` cobre 2+ cenários
  combinados, impacto individual de cada um, e o recálculo correto ao
  remover um cenário do conjunto ativo (assert antes/depois).
- Teste HTTP: `scenario_ids` vazio retorna só `base` com `simulation: null`;
  não-vazio retorna os três campos e os deltas batem com
  `simulation.saldo_final - base.saldo_final`.
- Navegação manual: adicionar 2+ cenários no `/relatorio-financeiro`,
  remover um, conferir recálculo instantâneo do saldo/menor saldo/gráfico
  sem reload de página.
