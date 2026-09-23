import { CreditCardOutlined } from "@ant-design/icons";
import { Skeleton } from "antd";

import type { Account } from "../../api/contracts";
import { CreditCardRow } from "./CreditCardRow";

export function CreditCardList({
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
      <div className="account-list-skeleton" role="status" aria-label="Carregando cartões de crédito">
        <Skeleton active paragraph={{ rows: 3 }} title={false} />
      </div>
    );
  }

  if (accounts.length === 0) {
    return (
      <div className="debt-list-empty">
        <CreditCardOutlined className="debt-list-empty-icon" aria-hidden="true" />
        <span>Nenhum cartão de crédito sincronizado ainda.</span>
      </div>
    );
  }

  return (
    <div className="credit-card-list" aria-label="Cartões de crédito">
      {accounts.map((account) => (
        <CreditCardRow key={account.id} account={account} onOpen={onOpen} />
      ))}
    </div>
  );
}
