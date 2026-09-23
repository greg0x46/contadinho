import { BankOutlined } from "@ant-design/icons";
import { Skeleton } from "antd";

import type { Account } from "../../api/contracts";
import { accountDisplayName, accountIdentityLine, formatAccountMoney } from "../../presentation/accountLabels";

/**
 * Bank accounts as flat rows: name + type/number on the left, the balance —
 * the one figure that matters here — as a plain right-aligned group. See
 * CreditCardList for the equivalent cards section.
 */
export function BankAccountList({
  accounts,
  isLoading,
  onOpen,
}: {
  accounts: Account[];
  isLoading: boolean;
  onOpen: (account: Account) => void;
}) {
  return (
    <section className="accounts-section" aria-label="Contas bancárias">
      <header className="accounts-section-header">
        <h2>Contas bancárias</h2>
        {!isLoading && accounts.length > 0 && (
          <small>
            {accounts.length} {accounts.length === 1 ? "conta" : "contas"}
          </small>
        )}
      </header>
      {isLoading ? (
        <div className="accounts-section-loading" role="status" aria-label="Carregando contas bancárias">
          <Skeleton active paragraph={{ rows: 2 }} title={false} />
        </div>
      ) : accounts.length === 0 ? (
        <div className="debt-list-empty">
          <BankOutlined className="debt-list-empty-icon" aria-hidden="true" />
          <span>Nenhuma conta bancária sincronizada ainda.</span>
        </div>
      ) : (
        <div className="accounts-list">
          {accounts.map((account) => (
            <button key={account.id} type="button" className="account-row" onClick={() => onOpen(account)}>
              <span className="account-row-identity">
                <span className="account-row-name">{accountDisplayName(account)}</span>
                <span className="account-row-meta">{accountIdentityLine(account) || "—"}</span>
              </span>
              <span className="account-row-balance">{formatAccountMoney(account.balance, account.currency_code)}</span>
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
