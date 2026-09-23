import { CreditCardOutlined } from "@ant-design/icons";
import { Skeleton } from "antd";

import type { Account } from "../../api/contracts";
import { formatOptionalDay } from "../../presentation/dates";
import {
  accountDisplayName,
  closingDaySourceHint,
  formatAccountMoney,
  maskedAccountNumber,
} from "../../presentation/accountLabels";
import { CreditUsageMeter } from "./CreditUsageMeter";

function totalLimit(accounts: Account[]): string {
  return accounts.reduce((sum, account) => sum + Number(account.credit_limit ?? "0"), 0).toFixed(2);
}

function closingHint(day: number | null): string | null {
  return day === null ? null : `Fechamento dia ${day}`;
}

/**
 * Credit cards as flat rows using the row's full width: identity on the
 * left, then three semantic groups — invoice, due date, available limit —
 * instead of every figure crammed into one corner. The card's total limit
 * isn't repeated per row; it's lower-weight information that belongs in the
 * section header instead. On a phone the groups stack under the identity.
 */
export function CreditCardList({
  accounts,
  isLoading,
  onOpen,
}: {
  accounts: Account[];
  isLoading: boolean;
  onOpen: (account: Account) => void;
}) {
  return (
    <section className="accounts-section" aria-label="Cartões de crédito">
      <header className="accounts-section-header">
        <h2>Cartões de crédito</h2>
        {!isLoading && accounts.length > 0 && (
          <small>Limite total {formatAccountMoney(totalLimit(accounts), "BRL")}</small>
        )}
      </header>
      {isLoading ? (
        <div className="accounts-section-loading" role="status" aria-label="Carregando cartões de crédito">
          <Skeleton active paragraph={{ rows: 2 }} title={false} />
        </div>
      ) : accounts.length === 0 ? (
        <div className="debt-list-empty">
          <CreditCardOutlined className="debt-list-empty-icon" aria-hidden="true" />
          <span>Nenhum cartão de crédito sincronizado ainda.</span>
        </div>
      ) : (
        <div className="accounts-list">
          {accounts.map((account) => {
            const numberMeta = maskedAccountNumber(account.number);
            const closing = closingHint(account.closing_day);
            return (
              <button key={account.id} type="button" className="card-row" onClick={() => onOpen(account)}>
                <span className="card-row-identity">
                  <span className="card-row-name">{accountDisplayName(account)}</span>
                  {numberMeta && <span className="card-row-meta">{numberMeta}</span>}
                </span>
                <span className="card-row-figures">
                  <span className="card-row-figure">
                    <span className="card-row-figure-head">
                      <span className="card-row-figure-label">Total em aberto</span>
                      <span className="card-row-figure-value">
                        {formatAccountMoney(account.balance, account.currency_code)}
                      </span>
                    </span>
                  </span>
                  <span className="card-row-figure">
                    <span className="card-row-figure-head">
                      <span className="card-row-figure-label">Vencimento</span>
                      <span className="card-row-figure-value">{formatOptionalDay(account.balance_due_date)}</span>
                    </span>
                    {closing && (
                      <span className="card-row-figure-hint" title={closingDaySourceHint(account.closing_day_source)}>
                        {closing}
                      </span>
                    )}
                  </span>
                  <span className="card-row-figure card-row-figure-limit">
                    <span className="card-row-figure-head">
                      <span className="card-row-figure-label">Limite disponível</span>
                      <span className="card-row-figure-value">
                        {formatAccountMoney(account.available_credit_limit, account.currency_code)}
                      </span>
                    </span>
                    <CreditUsageMeter ratio={account.credit_usage_ratio} />
                  </span>
                </span>
              </button>
            );
          })}
        </div>
      )}
    </section>
  );
}
