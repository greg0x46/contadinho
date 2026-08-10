import { FileTextOutlined } from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Alert, Typography } from "antd";

import type { AccountBill } from "../../api/contracts";
import { formatOptionalDay } from "../../presentation/dates";
import { formatAccountMoney } from "../../presentation/accountLabels";

export function AccountBillsTable({
  bills,
  isLoading,
  error,
}: {
  bills: AccountBill[];
  isLoading: boolean;
  error: unknown;
}) {
  const columns: ProColumns<AccountBill>[] = [
    {
      title: "Vencimento",
      dataIndex: "due_date",
      render: (_, bill) => formatOptionalDay(bill.due_date),
    },
    {
      title: "Fechamento",
      dataIndex: "closing_date",
      render: (_, bill) => formatOptionalDay(bill.closing_date),
    },
    {
      title: "Total",
      dataIndex: "total_amount",
      render: (_, bill) => (
        <span style={{ fontVariantNumeric: "tabular-nums" }}>
          {formatAccountMoney(bill.total_amount, bill.currency_code)}
        </span>
      ),
    },
    {
      title: "Pagamento mínimo",
      dataIndex: "minimum_payment_amount",
      render: (_, bill) => formatAccountMoney(bill.minimum_payment_amount, bill.currency_code),
    },
  ];

  return (
    <section>
      <Typography.Title level={4}>Faturas fechadas</Typography.Title>
      <Typography.Paragraph type="secondary">
        A instituição disponibiliza apenas faturas já fechadas — a fatura em aberto aparece no saldo
        do cartão.
      </Typography.Paragraph>
      {error !== null && error !== undefined && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as faturas desta conta."
          style={{ marginBottom: 16 }}
        />
      )}
      <ProTable<AccountBill>
        aria-label="Faturas fechadas"
        columns={columns}
        dataSource={bills}
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
              <FileTextOutlined className="debt-list-empty-icon" aria-hidden="true" />
              <span>Nenhuma fatura fechada sincronizada ainda.</span>
            </div>
          ),
        }}
      />
    </section>
  );
}
