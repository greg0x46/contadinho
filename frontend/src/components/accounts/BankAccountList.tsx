import { BankOutlined } from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Typography } from "antd";

import type { Account } from "../../api/contracts";
import {
  accountDisplayName,
  accountSubtypeLabel,
  formatAccountMoney,
} from "../../presentation/accountLabels";

export function BankAccountList({
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
      title: "Tipo",
      dataIndex: "account_subtype",
      render: (_, a) => accountSubtypeLabel(a.account_subtype) ?? "—",
    },
    { title: "Número", dataIndex: "number", render: (_, a) => a.number ?? "—" },
    {
      title: "Saldo",
      dataIndex: "balance",
      render: (_, a) => (
        <span style={{ fontVariantNumeric: "tabular-nums" }}>
          {formatAccountMoney(a.balance, a.currency_code)}
        </span>
      ),
    },
  ];

  return (
    <section>
      <Typography.Title level={4}>Contas bancárias</Typography.Title>
      <ProTable<Account>
        aria-label="Contas bancárias"
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
              <BankOutlined className="debt-list-empty-icon" aria-hidden="true" />
              <span>Nenhuma conta bancária sincronizada ainda.</span>
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
