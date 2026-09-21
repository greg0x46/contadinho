# Contadinho

Contadinho é um rastreador de finanças pessoais self-hosted, feito para uma
única pessoa acessar localmente ou em uma instalação online. Ele sincroniza a movimentação da
sua conta bancária e cartão de crédito através da API de Open Finance da
[Pluggy](#open-finance--pluggy) — que autoriza você a ler seus próprios
dados financeiros, não a gerenciar os de outras pessoas — e então permite
categorizar transações, automatizar a categorização recorrente com regras, e
acompanhar dívidas (como compras parceladas ou empréstimos) em relação às
transações que as quitam.

Ele roda como um único binário Go com o frontend React embutido dentro dele
— sem runtime de frontend em produção. O banco padrão é SQLite, com
Postgres opcional. Para acesso online, a hospedagem deve fornecer HTTPS.

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

O Contadinho concentra aplicação e frontend em um binário, com configuração
de autenticação independente da hospedagem:

- **Um único arquivo para rodar.** `go build` gera um único executável com
  o frontend já embutido. Sem Docker, sem proxy reverso, sem precisar do
  runtime do Node em produção (o Node só é necessário uma vez, para
  compilar o frontend).
- **Um único arquivo como banco de dados (por padrão).** Com SQLite —
  o padrão —, os dados e sessões ficam no arquivo `contadinho.db`.
  Preserve também a chave externa de criptografia ao mover ou restaurar
  a instalação. Sem servidor de banco de dados para instalar ou manter
  rodando. Para rodar várias instâncias do Contadinho compartilhando um
  banco na nuvem, Postgres é uma opção — veja
  [Rodando com Postgres](#rodando-com-postgres).
- **Configuração independente de hospedagem.** A conta é criada pelo terminal;
  a origem pública e a chave de criptografia são fornecidas por configuração.
  O app não depende de SDKs de nuvem ou serviços externos de autenticação.

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
- **Investimentos** — contas de investimento integradas e manuais, carteiras
  por objetivo, operações por ativo e avaliações manuais. Aportes e resgates
  conciliados com o extrato aparecem separados dos gastos, preservando seu
  efeito no caixa e evitando duplicação no patrimônio.
- **Autenticação por navegador** — login com e-mail e senha para a conta
  proprietária, sessões persistidas, saída e troca de senha. Sem cadastro público.
- **Segredos criptografados em repouso** — credenciais Pluggy protegidas com
  AES-256-GCM por uma chave independente da senha, fornecida pelo ambiente ou
  por arquivo. O worker funciona após reinícios sem exigir login.

## Investimentos por conta e objetivo

Em **Investimentos**, cadastre uma conta de custódia manual ou use o agrupamento
criado para uma conexão. Crie posições vazias para novas compras; informe saldo
inicial apenas para patrimônio que já existia. Compras aceitam quantidade,
preço, taxas e impostos. A opção de entrada junto com a compra permite registrar
um aporte ou reinvestir um rendimento em uma única ação.

No extrato, abra o lançamento e escolha **Vincular a investimento**. Confirme o
destino e a parcela; o restante continua disponível para outra movimentação.
É possível associar também o movimento importado do ativo. O botão
**Revisar lançamentos antigos** abre o histórico sem limite de período, sem
reclassificá-lo automaticamente.

Contas integradas mantêm seus saldos informados pela instituição. Use
**Vincular caixa da corretora** quando a conta financeira importada já representa
o mesmo caixa. Objetivos agrupam posições de qualquer instituição sem alterar
seu valor. Cotações manuais têm data; correções de operações recalculam os
custos subsequentes e são recusadas se produzirem caixa ou posição negativos.

Os contratos e limites estão na [referência de investimentos](.specs/contextos/investimentos/reference.md).

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
```

Antes de iniciar, configure a chave de criptografia e crie ou migre a conta.
O servidor recusa iniciar sem autenticação preparada ou com chave inválida.

### Deploy no homelab

Com a configuração persistente já instalada em
`/home/greg0x46/contadinho`, publique a versão atual com:

```sh
./deploy.sh
```

O script instala as dependências exatas do frontend, compila o binário para
Linux/amd64, envia-o por SSH para `greg0x46@192.168.0.196`, reinicia a unidade
`contadinho.service` e valida o endpoint `/health`. Se a validação falhar, ele
restaura o binário anterior e reinicia o serviço novamente. O banco, o arquivo
`contadinho.env` e a chave mestra não são alterados.

Host, diretório, unidade e URL de saúde podem ser sobrescritos por
`CONTADINHO_DEPLOY_HOST`, `CONTADINHO_DEPLOY_DIR`,
`CONTADINHO_DEPLOY_SERVICE` e `CONTADINHO_DEPLOY_HEALTH_URL`, respectivamente.
O reinício usa `sudo` no homelab e pode solicitar a senha do usuário remoto.

### Autenticação e chave de criptografia

Gere **uma única vez** uma chave de 32 bytes em Base64, em arquivo fora do
repositório. O exemplo recusa sobrescrever um arquivo existente:

```sh
mkdir -p "$HOME/.config/contadinho"
(umask 077; set -C; openssl rand -base64 32 > "$HOME/.config/contadinho/master.key")
export CONTADINHO_MASTER_KEY_FILE="$HOME/.config/contadinho/master.key"
export CONTADINHO_PUBLIC_URL="http://localhost:4200"
```

Alternativamente, forneça o Base64 em `CONTADINHO_MASTER_KEY` pelo mecanismo
de segredos da hospedagem. Defina exatamente uma das duas fontes. Não inclua
valores no repositório, argumentos, logs ou variáveis `VITE_*`.

`CONTADINHO_PUBLIC_URL` é a origem vista pelo navegador, sem caminho ou barra
final. Para uso online, use `https://seu-hostname`; HTTP é permitido somente
em `localhost`, `127.0.0.1` ou `::1`, para desenvolvimento. O HTTPS pode ser
terminado externamente; a aplicação não confia em headers de proxy para
escolher a política do cookie ou a origem permitida.

Para banco novo:

```sh
./contadinho auth init -db ./contadinho.db
./contadinho -db ./contadinho.db
```

O comando solicita e-mail e uma senha não vazia em terminal interativo, sem
eco da senha. Não há regras de tamanho ou composição: a escolha é do usuário.
Entre no navegador e configure as credenciais da Pluggy em
**Configurações**; o Item ID opcional cadastra uma nova conexão. Conexões
adicionais continuam disponíveis em **Open Banking**.

Para migrar seu banco existente:

1. Pare a versão antiga e faça backup consistente do banco.
2. Configure e preserve a nova chave.
3. Execute `./contadinho auth migrate -db ./contadinho.db`.
4. Informe a senha antiga de desbloqueio, seu e-mail e a nova senha de login.
5. Inicie o aplicativo com a mesma chave configurada.

A migração recriptografa todos os segredos e cria a conta numa transação;
falhas não deixam conversão parcial. O verificador antigo é removido e uma
segunda migração é recusada. Guarde o backup anterior para retorno à versão
antiga: após a migração, o binário antigo não consegue ler os segredos novos.
Não execute a migração com o aplicativo em funcionamento.

Preserve uma cópia segura da chave separada do backup do banco. A recuperação
da senha **não** recupera uma chave perdida. Trocar o arquivo por outra chave
não é rotação: o servidor recusará iniciar. Rotação de chave não faz parte
desta versão.

Para recuperar acesso pelo terminal, mantendo a criptografia:

```sh
./contadinho auth reset-password -db ./contadinho.db
```

Esse comando dispensa a chave, exige uma nova senha e revoga todas as sessões.
A troca em **Configurações** exige a senha atual e também encerra todas as
sessões. **Sair** encerra apenas a sessão do navegador atual.

Sessões duram no máximo sete dias, com expiração após 24 horas sem uso, e
sobrevivem a reinícios. Todos os acessos privados são verificados no backend.
Login aceita até cinco tentativas por minuto por e-mail e trinta por minuto
no processo; o limite reinicia junto com o processo. Escritas HTTP exigem
`Origin` igual à origem configurada e `X-Contadinho-Request: 1`.

Em **Configurações > Acesso**, o proprietário pode desativar a autenticação.
Desativá-la exige uma sessão válida; depois disso, qualquer pessoa que alcance
a instância pode consultar e alterar todos os dados sem login. Religá-la é
permitido a partir do modo aberto e faz as requisições seguintes voltarem a
exigir sessão imediatamente. As verificações de origem para escritas continuam
ativas nos dois modos. A preferência fica no banco; se não existir, o modo
protegido é usado, e se ela não puder ser lida a API fica indisponível — nunca
é aberta por falha de configuração.

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
SQLite. Os comandos `auth init`, `auth migrate` e `auth reset-password`
aceitam a mesma DSN na flag `-db` ou em `CONTADINHO_DB`. Sessões e conta ficam
no banco; a chave de criptografia permanece externa a ele.

O worker de sincronização em segundo plano assume que só uma instância o
executa por vez — hoje ele não coordena a reivindicação de execuções de
sincronização entre múltiplos processos/instâncias.

#### Documentar o schema com SchemaSpy

`docker-compose.schemaspy.yml` roda o [SchemaSpy](https://schemaspy.org/)
contra um Postgres de desenvolvimento acessível na máquina (`host.docker.internal`)
e grava o HTML em `schemaspy/output/` (diretório ignorado pelo git). Não é o
compose da aplicação. A senha é obrigatória; usuário, banco e porta têm
padrão `admin`, `contadinho` e `5432`:

```sh
SCHEMASPY_DB_PASSWORD=... docker compose -f docker-compose.schemaspy.yml up
# opcionais: SCHEMASPY_DB_USER, SCHEMASPY_DB_NAME, SCHEMASPY_DB_PORT
```

Depois abra `schemaspy/output/index.html`.

### Sincronização agendada

Como as rotas da API exigem sessão do navegador, um cron externo não consegue
mais disparar `POST /api/sync-runs`. Em vez disso, defina
`CONTADINHO_SYNC_SCHEDULE` para que o próprio processo enfileire, uma vez por
dia, uma sincronização de cada conexão ativa:

```sh
export CONTADINHO_SYNC_SCHEDULE="06:00"                  # horário local do processo
export CONTADINHO_SYNC_SCHEDULE="06:00 America/Sao_Paulo" # ou com fuso IANA explícito
```

Se o processo estiver parado no horário, a sincronização pendente é
enfileirada assim que ele subir; uma conexão que já sincronizou (inclusive
manualmente) depois do horário do dia não é repetida. Vazio desabilita.

### Rodar para desenvolvimento local

Depois de configurar a chave e preparar a conta com `go run ./cmd/contadinho auth init`
(ou `auth migrate` para banco antigo), o script abaixo inicia backend e Vite juntos. O frontend fica com hot
reload e as requisições de `/api` são encaminhadas automaticamente para o
backend:

```sh
./dev.sh
```

Abra a URL impressa pelo script na linha `Frontend:` (`http://127.0.0.1:5173`
por padrão). O script usa exatamente essa origem como padrão de
`CONTADINHO_PUBLIC_URL`, e o backend compara o header `Origin` do navegador
com ela de forma estrita: abrir por `http://localhost:5173`, ou já ter
exportado outro valor em `CONTADINHO_PUBLIC_URL`, resulta em 403
`invalid-origin` no login e em todo POST/PUT/DELETE. Se preferir outro host,
altere `VITE_DEV_HOST` (o script deriva a origem dele) em vez de exportar a
URL à mão. `Ctrl-C` encerra os dois processos. Para sobrescrever host e portas:

```sh
CONTADINHO_DEV_ADDR=localhost:8100 VITE_DEV_HOST=localhost VITE_DEV_PORT=5174 ./dev.sh
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
  auth/                                    conta proprietária, senhas e sessões por navegador
  settings/                                configurações criptografadas e migração dos segredos legados
  httpapi/                                   handlers HTTP e roteamento
  webui/                                       embute o frontend compilado
frontend/            SPA React/TypeScript (Vite)
```

Para o que cada um faz hoje (rotas, páginas, estado de implementação), ver
[`.specs/README.md`](.specs/README.md) e os `reference.md` de cada
contexto em `.specs/contextos/`.
