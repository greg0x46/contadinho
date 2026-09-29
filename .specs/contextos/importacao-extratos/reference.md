# Importação de extratos

## O que existe

Em **Contas e cartões → Importar extrato**, o usuário envia um CSV, escolhe
uma conta de arquivo existente ou cria uma nova, revisa a prévia por linha
e confirma. A tela mostra período, saldo, linhas novas, já importadas e
inválidas, avisos e histórico recente. Linhas inválidas só são ignoradas
quando o usuário escolhe explicitamente importar as válidas.

O primeiro formato aceito é o CSV Flash com as colunas `Data`, `Hora`,
`Movimentação`, `Valor`, `Meio de Pagamento` e `Saldo`. O parser considera o
fuso `America/Sao_Paulo` e valores decimais BRL exatos. O detector e o parser
ficam em `internal/statementimport`; novos formatos entram como adaptadores
registrados ali. O nome do arquivo não determina o período.

## Dados e regras

- Contas de arquivo usam `data_sources.provider = 'file'` e aparecem junto
  às demais contas. A agenda, o worker e a tela Open Banking processam apenas
  fontes `pluggy`.
- A prévia não grava dados. A confirmação reenvia o arquivo e confere SHA-256,
  formato, versão e classificação das linhas; uma mudança exige nova prévia.
- A identidade de cada movimento é derivada de conta, instante, descrição,
  valor, meio e ordinal entre linhas iguais. Isso evita duplicatas ao reenviar
  o arquivo ou enviar períodos sobrepostos. Saldo diferente para um movimento
  já importado é avisado para revisão; o lançamento anterior não é alterado.
- O saldo de conta avança somente com uma linha de instante posterior ao
  saldo atual e com saldo final não ambíguo. Diferenças de continuidade são
  exibidas como aviso; o sistema não cria movimentos para fechar diferenças.
- Confirmação grava execução, CSV bruto, conta, movimentos e eventos em uma
  transação de banco. O arquivo bruto fica no banco para auditoria, sem ser
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

O upload aceita até 2 MiB e 10 mil linhas. O primeiro formato é limitado a
contas de movimentação em BRL. Não há conciliação automática com transações
de contas Pluggy; possíveis coincidências exatas com transações Pluggy ou
com lançamento manual da mesma conta aparecem como aviso, sem fusão
automática.
