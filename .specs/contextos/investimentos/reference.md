# Investimentos

Contas de investimento representam a custódia; posições representam os ativos;
carteiras por objetivo agrupam posições sem criar um segundo saldo. Cada
posição pertence a uma única carteira ou fica sem objetivo.

## Domínio e APIs

`internal/investments` concentra contas, carteiras, posições manuais, operações,
avaliações e conciliação. As APIs usam `/api/investment-accounts`,
`/api/investment-portfolios`, `/api/investment-positions`,
`/api/investment-operations`, `/api/investment-reconciliations` e
`/api/investment-summary`. As consultas legadas `/api/investments` continuam
disponíveis para os ativos sincronizados e seu histórico importado.

Investimentos integrados mantêm os identificadores do provedor, agrupados
inicialmente pela conexão. Objetivos e conciliações são escolhas locais. Um
caixa de corretora já presente em `financial_accounts` pode ser vinculado,
sem adicionar novamente seu saldo ao patrimônio.

Operações manuais usam reais e aritmética decimal. Compras incorporam custos
ao custo médio ponderado; vendas baixam custo proporcional. Correções
reprocessam o histórico e rejeitam caixa ou quantidade negativos. Avaliações
são manuais e datadas; quando indisponíveis, o custo é identificado como custo,
sem inventar uma cotação ou rentabilidade. O livro guarda o custo total e o
valor total avaliado; custo médio e cotação unitária são derivados só para
exibição, de modo que valor e ganho não sofrem deriva decimal.

## Transferências e relatórios

Aporte e resgate transferem patrimônio. Compra e venda trocam caixa de
investimento por posição ou vice-versa. Rendimento recebido, taxa e imposto
têm natureza própria. Saldo inicial não é receita; valorização não é aporte.

Conciliações associam parcelas de lançamentos bancários às operações e,
quando disponível, ao histórico importado do investimento. A soma das parcelas
não pode exceder o lançamento. Sugestões exigem confirmação; o rótulo
`Investments` informado pelo provedor não basta para classificar um aporte.

`transactions.Item.EffectiveMoney` mantém o valor integral para extrato e
caixa. `InvestmentTransferAmount` identifica a parcela conciliada como
aporte/resgate; `ReportableAmount` identifica o restante elegível nos totais.
Uma conciliação integral recebe motivo `investment_transfer` apenas se o
lançamento ainda estava nos totais. Ignorado, categoria de transferência e
demais motivos já atribuídos têm precedência e são mantidos.
Gastos por categoria usam o restante; a curva de caixa preserva o movimento
integral. Os agregados mensais distinguem aportes e resgates.

Editar ou excluir um lançamento bancário manual conciliado exige desfazer
os vínculos primeiro. Ignorar um lançamento conciliado desfaz seus vínculos
(a operação volta a ser reportada sozinha), e um lançamento ignorado não pode
ser conciliado. A conciliação não altera saldos importados. Lançamentos
bancários manuais mantêm a regra de não modificar o saldo informado pelo banco.

## Patrimônio e limites

O patrimônio soma posições sincronizadas uma única vez, posições manuais e
caixa manual de investimento ainda não representado numa conta financeira
vinculada. Carteiras por objetivo não entram novamente na soma. Não são
inventadas valorizações para snapshots históricos sem dados.

Esta versão registra operações; não executa ordens ou transferências no banco.
Não inclui cotação automática, apuração fiscal, posições vendidas, câmbio nas
operações manuais ou agendamento de aportes recorrentes.

## Ações combinadas, revisão e filtros

`POST /api/investment-operations/batch` recebe `operations` (de 1 a 100 itens)
com o mesmo formato de uma operação, e `reconciliation` opcional. O vínculo
refere-se à primeira operação do lote. A gravação inteira é transacional:
uma compra inválida ou vínculo recusado não deixa aporte ou rendimento órfão.
A interface oferece aporte com compra e rendimento reinvestido. Quantidade e
preço calculam o valor bruto em decimal quando `amount` não é informado.

Contas integradas aceitam registros locais de aporte, resgate, rendimento,
taxa e imposto. Esses registros alimentam relatórios e conciliação; nunca
recalculam saldos ou posições do provedor. Compra, venda, saldo inicial e
cotação de posições integradas continuam sendo dados da instituição.

Um lançamento bancário também pode ser conciliado diretamente com um movimento
importado de conta integrada, sem registro manual: `POST
/api/investment-reconciliations` sem `operation_id` deriva uma operação
`source = synced` na custódia da conexão (aporte para `inflow`, rendimento para
`INTEREST`, resgate para os demais `outflow`), com valor e data do movimento.
A operação derivada é somente leitura, é reaproveitada por parcelas seguintes
do mesmo movimento (inclusive depois de desfeita a parcela que registrou o
movimento), some ao desfazer o último vínculo e nunca entra nos relatórios
por conta própria. Movimento sem direção, sem data ou fora de BRL
é recusado.

Uma transferência entre custódias leva o custo médio que a posição de origem
tinha na data da transferência; esse custo fica gravado na entrada de destino
e correções posteriores às compras de origem não o propagam.
Uma posição manual pode começar vazia para receber sua primeira compra.
A última cotação manual por unidade continua válida após compras e vendas;
a quantidade remanescente é avaliada por essa cotação, mantendo sua data.
Correções reprocessam o histórico na ordem de data, criação e identificador.

A conciliação confere direção, moeda BRL e capacidade restante de cada
lançamento, operação e movimento importado. Para contas integradas, o
movimento importado deve pertencer à mesma conexão. Parcelas concorrentes
são serializadas durante a validação. Sugestões na interface usam igualdade
do valor disponível e até cinco dias de diferença; exigem confirmação.
Taxas, impostos e rendimentos também podem ser vinculados, evitando duplicar
receitas e despesas. Desvincular restaura a parcela disponível.

A tela Investimentos oferece “Revisar lançamentos antigos”, abrindo o extrato
sem restrição de período. `POST /api/transactions/query` aceita
`filters.origin` (`manual`, `synced`). Nenhum histórico é reclassificado
automaticamente.

Posições aceitam filtros `account_id`, `portfolio_id`, `source`, `source_id`
e `include_closed`. Operações aceitam `account_id`, `position_id`,
`portfolio_id`, `source`, `source_id`, `from`, `to` e `reconciliation_state`
(`linked`, `unlinked`). As avaliações são operações `valuation` datadas,
consultáveis pelo mesmo histórico de operações.

O total da área de investimentos inclui o caixa efetivo das contas vinculadas.
No patrimônio esse mesmo caixa já está na parcela bancária, portanto não é
somado uma segunda vez. Na visão por objetivo, compras e vendas são rotuladas
como tal, sem confundi-las com as transferências de caixa da custódia.

Posições importadas em outras moedas mantêm a moeda e o valor originais.
Sem conversão cambial nesta versão, ficam fora dos totais de investimento,
metas e parcela de investimentos do patrimônio em reais. A interface sinaliza
quando existem posições fora desses totais.
