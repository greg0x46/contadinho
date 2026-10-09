# Importação de extratos

## O que existe

Em **Contas e cartões → Importar extrato**, o usuário envia um CSV, escolhe
uma conta de arquivo existente ou cria uma nova, revisa a prévia por linha
e confirma. A tela mostra período, saldo, linhas novas, já importadas e
inválidas, avisos e histórico recente. Linhas inválidas só são ignoradas
quando o usuário escolhe explicitamente importar as válidas. A tela oferece o
download de um CSV modelo, com linhas de exemplo fictícias, para quem precisa
montar o arquivo no formato aceito.

O primeiro formato aceito é o CSV Flash com as colunas `Data`, `Hora`,
`Movimentação`, `Valor`, `Meio de Pagamento` e `Saldo`. O parser considera o
fuso `America/Sao_Paulo` e valores decimais BRL exatos. O detector e o parser
ficam em `internal/statementimport`; novos formatos entram como adaptadores
registrados ali; cada adaptador também fornece o seu CSV modelo, montado a
partir do mesmo cabeçalho que o parser confere. O detector aceita o que o
parser aceita (BOM, CRLF, campos entre aspas e espaços após as vírgulas). O
nome do arquivo não determina o período.

## Dados e regras

- Contas de arquivo usam `data_sources.provider = 'file'` e aparecem junto
  às demais contas. A agenda, o worker e a tela Open Banking processam apenas
  fontes `pluggy`.
- A prévia não grava dados. A confirmação reenvia o arquivo e confere SHA-256,
  formato, versão e classificação das linhas; uma mudança exige nova prévia.
- A identidade de cada movimento é derivada de conta, instante, descrição,
  valor, meio, saldo final e ordinal entre linhas iguais. Isso evita duplicatas
  ao reenviar o arquivo ou enviar períodos sobrepostos: a repetição grava só
  uma nova execução e um novo registro de importação, sem lançamentos,
  decisões ou mudança de saldo. Uma linha com os mesmos dados textuais, mas
  saldo final diferente, é marcada como ambígua e exige decisão; a linha
  anterior não é alterada. O saldo da conta não avança nessa importação
  ambígua. A identidade antiga, sem saldo, ainda é reconhecida em reenvios de
  movimentos já importados quando o saldo coincide.
- Se uma linha idêntica (inclusive saldo) já veio de outro arquivo, a prévia
  marca a coincidência como ambígua. A confirmação exige uma escolha por linha:
  ignorar como repetição ou importar como outra compra. A nova identidade usa
  o arquivo e o número da linha, portanto reenviar o arquivo após essa escolha
  não cria outra compra. Se a classificação mudar entre prévia e confirmação,
  é exigida uma nova prévia.
- Linhas idênticas no mesmo arquivo são movimentos distintos (ordinal 1, 2,
  ...). Limite conhecido: um arquivo posterior com mais linhas idênticas do
  que o anterior importa só as excedentes.
- A identidade inclui a conta: o mesmo arquivo em duas contas gera
  lançamentos independentes.
- O saldo de conta avança somente com uma linha de instante posterior ao
  saldo atual e com saldo final não ambíguo. Ao importar somente as válidas,
  uma linha inválida com data ilegível ou não mais antiga que a última linha
  válida impede a atualização do saldo (os movimentos válidos entram
  normalmente), porque o saldo poderia ficar defasado; linhas inválidas mais
  antigas não interferem. Diferenças de continuidade são exibidas como aviso;
  o sistema não cria movimentos para fechar diferenças.
- Confirmação grava execução, CSV bruto, conta, movimentos e eventos em uma
  transação de banco; qualquer falha desfaz tudo (inclusive linhas já
  inseridas e a conta nova) e o mesmo envio pode ser repetido. O arquivo bruto fica no banco para auditoria, sem ser
  incluído em logs ou respostas. A execução fica separada das sincronizações
  Open Banking. A categorização aprendida e a automação são aplicadas aos
  lançamentos novos.
- Transações de arquivo aparecem com origem **Arquivo** no extrato e detalhe;
  o filtro de origem distingue Arquivo, Conexão automática e Manual.

## API

- `POST /api/statement-imports/preview` — upload multipart, `file`,
  `account_id` opcional, `format` opcional; retorna linhas, contagens,
  formato, versão, hash e impressão da prévia.
- `POST /api/statement-imports` — mesmo arquivo, conta existente ou nome de
  conta nova, hash/formato/versão/impressão esperados e `allow_partial`;
  retorna conta, execução e contagens gravadas.
- `GET /api/statement-imports/accounts` — contas de arquivo em BRL.
- `GET /api/statement-imports` — até 100 importações recentes.
- `GET /api/statement-imports/template` — CSV modelo (`format` opcional,
  padrão `flash_csv`), como anexo `modelo-extrato-flash.csv`; formato
  desconhecido responde 422.

Erros de leitura do arquivo respondem 422 com a mensagem do parser, upload
acima do limite responde 413 e falha de banco responde 503 sem detalhe
técnico (a causa vai só para o log).

O upload aceita até 2 MiB e 10 mil linhas. O primeiro formato é limitado a
contas de movimentação em BRL. Não há conciliação automática com transações
de contas Pluggy; possíveis coincidências exatas com transações Pluggy ou
com lançamento manual da mesma conta aparecem como aviso, sem fusão
automática: a linha do arquivo é gravada e as outras não são alteradas. O valor é comparado em todas as grafias com que pode estar
gravado (`-10`, `-10.0`, `-10.00`), porque Pluggy e lançamentos manuais
preservam a escala de origem.
