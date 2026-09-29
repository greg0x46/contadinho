# Importação de extratos de contas — plano (primeira entrega implementada)

O estado atual da primeira entrega está em
[`contextos/importacao-extratos/reference.md`](contextos/importacao-extratos/reference.md).
Este plano permanece para acompanhar extensões: novos formatos, revisão de
correções de saldo em movimentos repetidos, conciliação entre contas de
arquivo e Pluggy, e validação de migração com Postgres em ambiente integrado.

## Objetivo e recorte

Permitir importar transações de arquivos de instituições diferentes para o
mesmo motor de Lançamentos. O primeiro adaptador será o CSV do Flash. A
primeira entrega cobre contas de movimentação em BRL, upload manual, prévia,
confirmação, histórico e reimportação segura. Outros formatos devem exigir um
novo adaptador, sem alterar as regras de persistência ou a interface de
revisão. Não incluir faturas de cartão de crédito, OFX nem conciliação
automática com transações da Pluggy nesta entrega.

O arquivo real recebido foi usado apenas para analisar o formato. Não
versioná-lo nem usá-lo como fixture; ele contém dados financeiros pessoais.

## Evidências do CSV Flash recebido

- Codificação UTF-8 com BOM, separador vírgula e cabeçalho `Data, Hora,
  Movimentação, Valor, Meio de Pagamento, Saldo`.
- 19 linhas; datas entre 08/09/2026 e 25/09/2026; todas as seis colunas
  preenchidas. Valor assinado em reais com vírgula decimal e espaço não
  separável (`-R$ 49,90`); horário no formato `HH:mm`.
- Há saídas por `Cartão` e `Pagamento PIX`, e entradas por `Depósito`. Neste
  exemplo, a diferença entre saldos de linhas adjacentes corresponde ao
  valor da linha mais recente. O arquivo aparece em ordem decrescente, mas
  o parser não deve depender dessa ordem.
- O nome contém `06-2026`, embora as linhas sejam de setembro de 2026.
  Data, período e saldo devem vir do conteúdo, nunca do nome do arquivo.
- Nenhuma coluna fornece um identificador de transação ou de conta. Um
  extrato com saldos não prova, sozinho, que a conta é cartão de crédito;
  tratá-la como conta de movimentação até existir evidência em contrário.

## Contrato do importador

1. `Detector` reconhece formato por conteúdo, cabeçalho e codificação;
   formato explícito opcional pode resolver ambiguidades. Versão do
   adaptador faz parte do resultado.
2. `Parser` recebe bytes e produz linhas normalizadas com número da linha,
   instante no fuso `America/Sao_Paulo`, descrição original, valor decimal
   exato assinado, moeda, meio de pagamento, saldo após a movimentação e
   eventuais erros/avisos. Nenhum adaptador escreve no banco.
3. O serviço comum recebe conta de destino, linhas normalizadas e hash do
   arquivo. Ele calcula identidades, classifica linhas como novas,
   já importadas ou inválidas, e gera a mesma prévia que será revalidada na
   confirmação. O parser Flash converte `dd/mm/aaaa`, `HH:mm`, `R$` e vírgula
   decimal sem `float`.
4. O arquivo inteiro deve falhar se cabeçalho, codificação, estrutura ou
   associação à conta forem ambíguos. Linhas inválidas devem aparecer com
   número e motivo na prévia; confirmação só com escolha explícita de
   importar as válidas, ou após corrigir o arquivo. Não criar transações
   silenciosamente incompletas.

## Identidade, duplicidade e saldo

- A conta importada tem identidade local persistente, escolhida na primeira
  importação e reutilizada nas seguintes. Não derivar conta do nome do
  arquivo. Na primeira entrega, criar uma conta de arquivo ou escolher uma
  conta previamente criada para arquivos; não gravar diretamente numa
  conta Pluggy, pois isso permitiria duplicar transações e disputar saldo.
- Usar `data_sources.provider = 'file'` e um identificador local estável,
  distinto de `format = 'flash_csv'`. A agenda e o botão de sincronização
  Pluggy devem selecionar apenas fontes Pluggy; o worker não deve reivindicar
  execuções de arquivo.
- Sem ID nativo, a identidade provisória de cada lançamento é um hash
  versionado de conta + data/hora + descrição normalizada + valor + meio de
  pagamento, com ordinal para ocorrências idênticas. Ignorar nome, hash do
  arquivo e posição da linha para permitir arquivos sobrepostos/reordenados.
  O ordinal preserva multiplicidade, mas não identifica uma correção de
  descrição/valor: a prévia deve sinalizar colisões ou possíveis correções
  para revisão, sem substituição automática de transações já categorizadas.
  `Saldo` entra no hash de conteúdo, não na identidade, para permitir corrigir
  esse dado sem criar outra transação. A mesma conta + arquivo pode ser
  reenviada sem inserir duplicatas.
- Saldos adjacentes devem ser verificados quando o extrato permitir. Uma
  quebra de continuidade vira aviso, pois pode haver recorte ou movimentos
  ausentes; jamais inventar lançamentos para fechar a diferença. O saldo da
  conta só avança com o saldo da transação mais recente conhecida, guardando
  seu instante de referência; importar um arquivo antigo não o regride.
- Mostrar possíveis correspondências com lançamentos manuais/Pluggy como
  aviso para revisão; não fundir por semelhança. Preservar decisões de
  categoria, inclusão e vínculos de lançamentos já existentes.

## Persistência e API

- Reaproveitar `financial_accounts`, `financial_transactions`,
  `data_sources`, `sync_runs`, `raw_imports` e `normalization_events` para
  integrar consulta, automação, balanço e trilha de auditoria. Criar
  migrações equivalentes em SQLite e Postgres para representar a origem
  arquivo, metadados de importação e `balance_as_of` da conta, e ajustar os
  campos/checks de `raw_imports` para upload. O CSV bruto só é guardado na
  confirmação, associado à execução, com hash SHA-256; não registrá-lo em
  logs ou respostas da API.
- Extrair da sincronização Pluggy a persistência comum de conta/transação,
  hash normalizado, eventos, contadores e hooks de categorização/automação.
  O adaptador de arquivo não deve simular respostas da API Pluggy. Uma
  confirmação deve ser atômica: arquivo, execução, conta, transações e
  eventos entram juntos, ou nada entra. Hooks pós-gravação precisam ter
  estratégia de repetição que não duplique lançamentos.
- `POST /api/statement-imports/preview`: multipart com arquivo, formato
  opcional e conta de destino opcional; devolve formato detectado, período,
  moeda, contagens, linhas com erros/avisos e amostra limitada. Não persiste.
- `POST /api/statement-imports`: reenvia o arquivo e confirma a conta,
  versão/formato e hash vistos na prévia; revalida tudo sob transação e
  devolve execução + contagens efetivas. Divergência entre prévia e
  confirmação deve retornar conflito legível, não importar às cegas.
  Listagem/detalhe do histórico podem reutilizar a API de execuções, com
  filtro de tipo, desde que não confundam importação com sincronização.
- Limitar tamanho e número de linhas do upload; exigir autenticação já
  usada nas rotas financeiras; mensagens de erro sem conteúdo financeiro.

## Experiência no frontend

Fluxo: selecionar CSV → detectar formato → escolher/criar conta e conferir
período, saldo, novas/repetidas/inválidas → confirmar → ver resultado e
histórico. Mostrar aviso explícito quando houver linhas inválidas, possíveis
duplicatas com outras origens ou diferença nos saldos. Identificar a origem
do lançamento como arquivo no detalhe e nos filtros existentes, mantendo as
ações de categorização e inclusão.

## Etapas de implementação e aceite

1. **Base de domínio e migrações:** registrar fontes/contas de arquivo e
   distinguir execuções manuais das agendadas. Aceite: Pluggy continua
   sincronizando; uma fonte de arquivo nunca é enviada ao worker.
2. **Parser Flash e prévia:** parser isolado, valores exatos, datas e fusos,
   erros por linha, validação de saldo e detecção de duplicidade. Aceite:
   fixture sintética com cabeçalho/BOM e variações de saída/entrada; teste
   para nome de arquivo com período enganoso e linhas reordenadas.
3. **Confirmação e auditoria:** transação atômica, identidade estável,
   reimportação, sobreposição de arquivos, saldo com data de referência,
   eventos e hooks. Aceite: upload repetido insere zero transações; arquivo
   antigo não regride saldo; falha no meio não deixa importação parcial;
   SQLite e Postgres obedecem ao mesmo contrato.
4. **Interface:** prévia e confirmação com erros e histórico, incluindo
   estados de conflito. Aceite: fluxo completo em conta nova e existente
   de arquivo, com contagens iguais às gravadas após revalidação.

Antes de implementar, confirmar em amostras adicionais se o Flash sempre
exporta o mesmo cabeçalho/colunas e se o saldo de cada linha representa
saldo após a movimentação. O primeiro CSV sustenta essas hipóteses, mas não
garante todos os formatos de exportação da instituição.
