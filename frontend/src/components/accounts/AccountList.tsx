import { BankOutlined } from "@ant-design/icons";
import { Skeleton } from "antd";

import type { Account } from "../../api/contracts";
import { AccountRow } from "./AccountRow";

export function AccountList({
  accounts,
  isLoading,
  onOpen,
}: {
  accounts: Account[];
  isLoading: boolean;
  onOpen: (account: Account) => void;
}) {
  if (isLoading) {
    return (
      <div className="account-list-skeleton" role="status" aria-label="Carregando contas bancárias">
        <Skeleton active paragraph={{ rows: 3 }} title={false} />
      </div>
    );
  }

  if (accounts.length === 0) {
    return (
      <div className="debt-list-empty">
        <BankOutlined className="debt-list-empty-icon" aria-hidden="true" />
        <span>Nenhuma conta bancária sincronizada ainda.</span>
      </div>
    );
  }

  return (
    <div className="account-list" aria-label="Contas bancárias">
      {accounts.map((account) => (
        <AccountRow key={account.id} account={account} onOpen={onOpen} />
      ))}
    </div>
  );
}
