# `.specs/` — guia de navegação

Ponto de entrada para quem (humano ou LLM) chega neste repositório sem
contexto e precisa entender o domínio antes de mexer em código.

## Ordem de leitura sugerida

1. **`/README.md`** (raiz) — o que é o Contadinho, como rodar, stack.
2. **Este arquivo** — o mapa do que tem aqui dentro.
3. **`motores-de-dominio.md`** — as abstrações de domínio reutilizáveis
   (nível ideal/arquitetura), e os princípios que as conectam.
4. **`contextos/<contexto>/reference.md`** — o que existe e funciona hoje,
   feature por feature. Comece pelo contexto que você vai tocar.
5. O spec de design daquele contexto, se existir (linkado a partir do
   `reference.md`) — o racional/histórico da decisão, não repetido aqui.
   Hoje nenhum contexto tem um; ver convenção abaixo para quando criar um.

## Os tipos de doc nesta pasta

### `motores-de-dominio.md`

Doc único, guarda-chuva de **arquitetura**, não de feature. Descreve o
domínio no nível ideal — onde o código diverge disso, a divergência é dita
explicitamente ali, ou apontada a partir do `reference.md` do contexto
correspondente.

### `contextos/<contexto>/reference.md`

Um arquivo por contexto/feature (`contextos/pendencias/`,
`contextos/relatorio-financeiro/`, etc.), descrevendo o **estado atual,
literal**: o que o backend/frontend fazem hoje, pacotes envolvidos, rotas
HTTP, páginas do frontend. É a fonte de verdade para "isso já foi
implementado?".

Diferente dos specs de design (próxima seção), estes são **docs vivos** —
devem ser atualizados sempre que a feature correspondente muda, no mesmo
commit/PR da mudança. Um `reference.md` desatualizado é pior que nenhum.

Contextos existentes hoje: `sincronizacao-open-banking`, `transacoes`,
`categorias`, `automacao`, `pendencias`, `cenarios`, `recorrencias`,
`relatorio-financeiro`, `patrimonio-liquido`, `configuracoes-autenticacao`.

### Specs de design soltos em `.specs/` (`<feature>.md` [+ `<feature>/mN-*.md`])

Nenhum existe no momento — os specs de feature anteriores
(`relatorio-financeiro.md` e milestones, `plano-pagamento-e-cenarios-
projecao.md`) foram removidos por já estarem cobertos pelos `reference.md`
correspondentes (`contextos/relatorio-financeiro/`, `contextos/pendencias/`,
`contextos/cenarios/`). O formato continua sendo a convenção para a
**próxima** feature de escopo razoável, quando fizer sentido registrar o
racional de uma decisão de design num momento específico — motivação,
alternativas descartadas, decisões de arquitetura, plano de execução por
milestone. Enquanto um spec desses está em aberto (feature ainda não
implementada, ou parcialmente), ele é a referência; assim que a feature é
concluída, o conteúdo relevante migra para o `reference.md` do contexto e o
spec de design é removido — evita manter dois documentos que podem
divergir sobre o mesmo estado atual.

## Blocos transversais do frontend

Não pertencem a um contexto específico — reaproveitados por vários:

- `frontend/src/components/shared/WidgetCard.tsx` + `SummaryCard.tsx` —
  shell padrão de card com loading/erro/vazio.
- `frontend/src/theme/tokens.ts` — cores/fonte/spacing/tema antd.
- `frontend/src/components/filters/filterUrl.ts` — serialização de filtros
  para `URLSearchParams` (filtros de transações, seleção de cenários).
- `frontend/src/presentation/money.ts` — formatação/aritmética de dinheiro
  sobre strings decimais.
- Recharts é a biblioteca de gráficos (Relatório Financeiro, Patrimônio
  Líquido).

Testes: Vitest + Testing Library cobrem o frontend hoje (colocalizados,
`*.test.ts(x)`). Playwright está scaffolded no `package.json`
(`test:e2e`) mas sem `playwright.config.*` nem testes — não rode
`npm run test:e2e` esperando que funcione.

## Convenção para trabalho novo

- **Feature nova de escopo razoável, com decisões de design a registrar**:
  crie `.specs/<feature>.md` (quebre em `.specs/<feature>/mN-*.md` se for
  grande o bastante para ter milestones independentes). Declare no topo se
  já foi implementado ou não.
- **Ao concluir a implementação de qualquer feature** (nova ou existente):
  atualize (ou crie) `contextos/<contexto>/reference.md` no mesmo PR, e
  remova o spec de design que tiver sido usado para planejá-la (ver seção
  acima). Se a feature for visível o bastante para entrar na lista de
  funcionalidades da raiz, atualize também a seção "Funcionalidades" do
  `/README.md`.
- **Mudança pequena/local dentro de um contexto já documentado**: só
  atualize o `reference.md` existente — não precisa de spec de design novo
  para toda alteração.
