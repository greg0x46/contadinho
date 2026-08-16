import { CreditCardOutlined } from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { formatOptionalDay } from "../../presentation/dates";
import { Tooltip, Typography } from "antd";

import type { Account } from "../../api/contracts";
import {
  accountDisplayName,
  closingDayLabel,
  closingDaySourceHint,
  formatAccountMoney,
} from "../../presentation/accountLabels";
import { CreditUsageMeter } from "./CreditUsageMeter";

export function CreditCardList({
  accounts,
  isLoading,
  onOpen,
}: {
  accounts: Account[];
  isLoading: boolean;
  onOpen: (account: Account) => void;
}) {
  const columns: ProColumns<Account>[] = [
    { title: "Nome", dataIndex: "name", render: (_, a) => accountDisplayName(a) },
    { title: "Instituição", dataIndex: "institution", render: (_, a) => a.institution ?? "—" },
    {
      title: "Fatura atual",
      dataIndex: "balance",
      render: (_, a) => (
        <Tooltip title="Saldo devedor informado pela instituição, incluindo a fatura ainda em aberto">
          <span style={{ fontVariantNumeric: "tabular-nums" }}>
            {formatAccountMoney(a.balance, a.currency_code)}
          </span>
        </Tooltip>
      ),
    },
    {
      title: "Limite",
      dataIndex: "credit_limit",
      render: (_, a) => (
        <span style={{ fontVariantNumeric: "tabular-nums" }}>
          {formatAccountMoney(a.credit_limit, a.currency_code)}
        </span>
      ),
    },
    {
      title: "Uso do limite",
      dataIndex: "credit_usage_ratio",
      render: (_, a) =>
        a.credit_usage_ratio === null ? "—" : <CreditUsageMeter ratio={a.credit_usage_ratio} />,
    },
    {
      title: "Fechamento",
      dataIndex: "closing_day",
      render: (_, a) => (
        <Tooltip title={closingDaySourceHint(a.closing_day_source)}>
          <span>{closingDayLabel(a.closing_day)}</span>
        </Tooltip>
      ),
    },
    {
      title: "Vencimento",
      dataIndex: "balance_due_date",
      render: (_, a) => formatOptionalDay(a.balance_due_date),
    },
  ];

  return (
    <section>
      <Typography.Title level={4}>Cartões de crédito</Typography.Title>
      <ProTable<Account>
        aria-label="Cartões de crédito"
        columns={columns}
        dataSource={accounts}
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
              <CreditCardOutlined className="debt-list-empty-icon" aria-hidden="true" />
              <span>Nenhum cartão de crédito sincronizado ainda.</span>
            </div>
          ),
        }}
        onRow={(account) => ({
          onClick: () => onOpen(account),
          style: { cursor: "pointer" },
        })}
      />
    </section>
  );
}
