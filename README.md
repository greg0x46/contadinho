# Contadinho

Contadinho é um rastreador de finanças pessoais self-hosted, feito para uma
única pessoa rodar no próprio computador. Ele sincroniza a movimentação da
sua conta bancária e cartão de crédito através da API de Open Finance da
[Pluggy](#open-finance--pluggy) — que autoriza você a ler seus próprios
dados financeiros, não a gerenciar os de outras pessoas — e então permite
categorizar transações, automatizar a categorização recorrente com regras, e
acompanhar dívidas (como compras parceladas ou empréstimos) em relação às
transações que as quitam.

Ele roda como um único binário Go com o frontend React embutido dentro dele
— não há servidor de frontend separado, proxy reverso ou container para
rodar em produção. Os dados ficam em um único arquivo SQLite local.

## Documentação de domínio e contribuindo

Contribuições, features novas e refactors são bem-vindos. Antes de mexer em
uma área do domínio, veja [`.specs/README.md`](.specs/README.md) — o hub de
navegação da pasta `.specs/`, com a arquitetura de domínio
(`motores-de-dominio.md`), o estado atual de cada feature
(`contextos/<contexto>/reference.md`) e o racional histórico das decisões
de design. Ao concluir uma feature nova ou alterar uma existente,
atualize o `reference.md` do contexto correspondente (e esta seção
"Funcionalidades" abaixo, se for relevante o bastante) no mesmo PR — specs
desatualizados atrapalham o próximo colaborador tanto quanto código
desatualizado.

## Por que um binário único

Rodar o Contadinho não exige instalar ou configurar nada além do próprio
binário:

- **Um único arquivo para rodar.** `go build` gera um único executável com
  o frontend já embutido. Sem Docker, sem proxy reverso, sem precisar do
  runtime do Node em produção (o Node só é necessário uma vez, para
  compilar o frontend).
- **Um único arquivo como banco de dados (por padrão).** Com SQLite —
  o padrão —, todo o estado da aplicação é o arquivo `contadinho.db` —
  copie, faça backup, mova para outro computador, apague, como qualquer
  outro arquivo. Sem servidor de banco de dados para instalar ou manter
  rodando. Para rodar várias instâncias do Contadinho compartilhando um
  banco na nuvem, Postgres é uma opção — veja
  [Rodando com Postgres](#rodando-com-postgres).
- **Zero configuração para começar.** As flags `-addr` e `-db` existem para
  permitir sobrescrever os padrões, mas nada exige isso — `./contadinho`
  sozinho já sobe um servidor funcional em `localhost:4200`, gravando em
  `./contadinho.db` ao lado do binário. A configuração inicial (senha,
  credenciais da Pluggy) acontece pelo navegador, na primeira vez que você
  abre o app — não existe arquivo `.env` para editar manualmente.

## Funcionalidades

- **Sincronização Open Banking** — um worker em segundo plano consulta a
  Pluggy periodicamente em busca de novas transações e dados de conta,
  registrados como execuções de sincronização auditáveis (histórico,
  métricas, falhas) em vez de uma importação caixa-preta.
- **Transações** — lista de transações pesquisável/filtrável, inclusão/
  exclusão manual (ex.: ignorar um estorno ou uma duplicata) e
  categorização. Lançamentos também podem ser criados à mão numa conta já
  existente — mesmas regras de categorização e automação de um lançamento
  sincronizado, com edição e exclusão restritas a esses lançamentos
  manuais.
- **Regras de automação** — regras baseadas em condições que categorizam
  novas transações automaticamente e podem ser aplicadas retroativamente às
  já existentes.
- **Pendências (dívidas e recebíveis)** — acompanhe um total real (dívida,
  compra parcelada, recebível) e vincule as transações que o quitam, com
  regras de elegibilidade para quais transações podem ser vinculadas.
- **Categorias** — um catálogo de categorias definido pelo usuário, com
  histórico de categorização.
- **Recorrências** — cadastre compromissos de fluxo de caixa conhecidos de
  antemão (salário, aluguel, assinaturas), reconciliados contra as
  transações reais que os quitam via regra de automação.
- **Cenários** — projeções hipotéticas autoradas pelo usuário ("e se eu
  viajar", "e se eu trocar de emprego"), ou o plano de parcelas de uma
  dívida/recebível — nunca contaminam totais reais a menos que
  explicitamente incluídos numa consulta.
- **Relatório financeiro** — navegação temporal única (Realizado → Hoje →
  Projeção Base → Simulação): quanto você terá numa data futura, qual seu
  menor saldo até lá, e como cenários hipotéticos mudam essa resposta.
- **Patrimônio líquido** — snapshots e histórico de patrimônio (ativos −
  passivos) ao longo do tempo.
- **Segredos criptografados em repouso** — as credenciais da Pluggy e
  outras configurações sensíveis ficam armazenadas no SQLite criptografadas
  com AES-256-GCM, usando uma chave derivada (Argon2id) de uma senha
  definida na primeira execução. A chave existe apenas na memória do
  processo do servidor; o app volta a ficar bloqueado sempre que o processo
  reinicia.

## Stack técnica

- **Backend**: Go, `net/http` (roteamento da stdlib, sem framework), SQLite
  via [`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite) (sem cgo) como
  padrão, com Postgres opcional via [`pgx`](https://github.com/jackc/pgx) —
  veja [Rodando com Postgres](#rodando-com-postgres) — e migrações via
  [`goose`](https://github.com/pressly/goose) para os dois dialetos.
- **Frontend**: React 19, TypeScript, [Ant Design](https://ant.design/) /
  Pro Components, [TanStack Query](https://tanstack.com/query), Vite.
- **Testes**: pacote `testing` padrão do Go, [Vitest](https://vitest.dev/) +
  Testing Library para componentes, [Playwright](https://playwright.dev/)
  para end-to-end.

## Como começar

### Pré-requisitos

- Go 1.26+
- Node.js 24+
- Uma conta na [Pluggy](https://pluggy.ai) com um client ID/secret e ao menos
  um `item_id` da conta que você quer sincronizar — veja o guia da Pluggy
  [Get your API keys](https://docs.pluggy.ai/docs/get-your-api-keys), e a
  seção [Open Finance & Pluggy](#open-finance--pluggy) abaixo. Bancos
  adicionais são cadastrados depois, em `/open-banking`, colando o `item_id`
  de cada um — o client ID/secret é o mesmo para todos.

### Rodar a build de produção (binário único)

```sh
./build.sh
./contadinho
```

Por padrão, o servidor escuta em `http://localhost:4200` (`-addr` para
sobrescrever) e guarda o banco de dados em `./contadinho.db` (`-db` para
sobrescrever). Na primeira requisição, abra o app no navegador e complete a
tela de configuração — ela pede uma senha de desbloqueio e suas credenciais
da Pluggy, que em seguida são criptografadas e armazenadas.

### Rodando com Postgres

SQLite continua sendo o padrão para uso local. Para rodar contra um Postgres
compartilhado — por exemplo, múltiplas instâncias do Contadinho apontando
para o mesmo banco na nuvem —, passe uma DSN `postgres://` (ou
`postgresql://`) na flag `-db` em vez de um caminho de arquivo:

```sh
./contadinho -db "postgres://usuario:senha@host:5432/contadinho?sslmode=require"
```

O driver é detectado automaticamente pelo prefixo da DSN. As migrações do
schema Postgres são aplicadas automaticamente, do mesmo jeito que as do
SQLite — nenhuma etapa manual de setup do banco é necessária além de ele
existir e estar acessível. Cada instância ainda precisa do próprio
`POST /api/unlock` (ou tela de configuração) na primeira vez que atende uma
requisição — a chave de criptografia é derivada da senha de forma
determinística, então qualquer instância com a senha correta decifra o que
outra instância gravou, mas o estado "desbloqueado" em si vive na memória de
cada processo, não é compartilhado entre eles.

O worker de sincronização em segundo plano assume que só uma instância o
executa por vez — hoje ele não coordena a reivindicação de execuções de
sincronização entre múltiplos processos/instâncias.

### Rodar para desenvolvimento local

O script abaixo inicia o backend e o Vite juntos. O frontend fica com hot
reload e as requisições de `/api` são encaminhadas automaticamente para o
backend:

```sh
./dev.sh
```

Abra `http://localhost:5173`. `Ctrl-C` encerra os dois processos. Para
sobrescrever as portas:

```sh
CONTADINHO_DEV_ADDR=localhost:8100 VITE_DEV_PORT=5174 ./dev.sh
```

Se as dependências ainda não estiverem instaladas, execute `cd frontend &&
npm install` uma vez antes de iniciar o script.

### Testes e verificações

```sh
# backend
go build ./...
go vet ./...
go test ./...

# frontend
cd frontend
npm run lint
npm run typecheck
npm test
```

`npm run test:e2e` (Playwright) está declarado no `package.json` mas ainda
não tem `playwright.config.*` nem testes — scaffolded, não implementado.

## Open Finance & Pluggy

Este projeto só existe por causa da regulamentação de Open Finance no
Brasil e da [Pluggy](https://pluggy.ai), a provedora de infraestrutura de
Open Finance cuja API o Contadinho integra. A Pluggy se conecta a mais de
130 instituições financeiras brasileiras e, como Iniciadora de Transação de
Pagamento (ITP) regulada pelo Banco Central, oferece a aplicações como esta
uma forma padronizada e autorizada de ler saldos de conta, transações e
dados de investimento em nome do usuário — transformando o "meus dados são
meus" de slogan em algo que um desenvolvedor independente consegue de fato
usar como base, sem precisar de uma integração sob medida com cada
instituição.

Todo o crédito à equipe da Pluggy por essa infraestrutura e por viabilizar
o acesso a Open Finance para projetos pequenos/independentes. Veja
[pluggy.ai](https://pluggy.ai) para saber mais sobre a plataforma, e
[Get your API keys](https://docs.pluggy.ai/docs/get-your-api-keys) para o
passo a passo de como obter o client ID/secret que este projeto pede
durante a configuração.

## Estrutura do projeto

```
cmd/contadinho/     ponto de entrada: conecta DB, servidor HTTP e worker em segundo plano
internal/
  db/                conexão SQLite/Postgres e migrações
  pluggy/             cliente da API da Pluggy e mapeamento de dados
  syncsvc/             orquestração das execuções de sincronização
  worker/              loop de polling de sincronização em segundo plano
  money/                primitivas de domínio compartilhadas (classificação, valor efetivo)
  transactions/           consulta de transações e estado de inclusão
  categories/              catálogo de categorias e categorização
  rules/                     núcleo de matching combinável (condições/operadores)
  automation/                 motor de regras de automação, sobre internal/rules
  recurrences/                   compromissos recorrentes, reconciliados via automation
  payables/                        dívidas/recebíveis e vínculo de transações que os quitam
  scenarios/                         projeções hipotéticas e planos de pagamento
  timeline/                            funde lançamentos + recorrências + cenários numa série única
  networth/                              snapshots de patrimônio líquido
  settings/                                armazenamento de configurações criptografadas, autenticação, sessões
  httpapi/                                   handlers HTTP e roteamento
  webui/                                       embute o frontend compilado
frontend/            SPA React/TypeScript (Vite)
```

Para o que cada um faz hoje (rotas, páginas, estado de implementação), ver
[`.specs/README.md`](.specs/README.md) e os `reference.md` de cada
contexto em `.specs/contextos/`.
