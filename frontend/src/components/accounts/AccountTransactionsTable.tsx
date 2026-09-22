import { SwapOutlined } from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Alert, Typography } from "antd";
import { Link } from "react-router-dom";

import type { TransactionItem } from "../../api/contracts";
import { formatOptionalLocalDay } from "../../presentation/dates";
import { formatSignedBRL } from "../../presentation/money";
import { maskedCardNumber } from "../../presentation/accountLabels";

export function AccountTransactionsTable({
  transactions,
  isLoading,
  error,
  accountId,
}: {
  transactions: TransactionItem[];
  isLoading: boolean;
  error: unknown;
  accountId: string;
}) {
  const columns: ProColumns<TransactionItem>[] = [
    {
      title: "Data",
      dataIndex: "occurred_at",
      render: (_, item) => formatOptionalLocalDay(item.occurred_at),
    },
    {
      title: "Descrição",
      dataIndex: "description",
      render: (_, item) => item.description ?? "Sem descrição",
    },
    {
      title: "Categoria",
      dataIndex: "internal_category",
      render: (_, item) => item.internal_category?.name ?? "Sem categoria",
    },
    {
      title: "Cartão",
      dataIndex: "card",
      render: (_, item) => (item.card === null ? "—" : maskedCardNumber(item.card.number)),
    },
    {
      title: "Valor",
      dataIndex: "effective_money",
      render: (_, item) =>
        item.effective_money === null ? (
          "—"
        ) : (
          <span style={{ fontVariantNumeric: "tabular-nums" }}>
            {formatSignedBRL(item.effective_money.value, item.classification)}
          </span>
        ),
    },
  ];

  return (
    <section>
      <Typography.Title level={4}>Últimas transações</Typography.Title>
      {error !== null && error !== undefined && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as transações desta conta."
          style={{ marginBottom: 16 }}
        />
      )}
      <ProTable<TransactionItem>
        aria-label="Últimas transações"
        columns={columns}
        dataSource={transactions}
        loading={isLoading}
        rowKey="id"
        search={false}
        options={false}
        pagination={false}
        cardBordered
        scroll={{ x: "max-content" }}
        locale={{
          emptyText: (
            <div className="debt-list-empty">
              <SwapOutlined className="debt-list-empty-icon" aria-hidden="true" />
              <span>Nenhuma transação sincronizada para esta conta.</span>
            </div>
          ),
        }}
      />
      <Typography.Paragraph style={{ marginTop: 12 }}>
        <Link to={`/transacoes?account_ids=${encodeURIComponent(accountId)}`}>
          Ver todas as transações
        </Link>
      </Typography.Paragraph>
    </section>
  );
}
