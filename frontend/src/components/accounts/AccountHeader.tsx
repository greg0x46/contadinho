import { Tooltip } from "antd";

import type { Account } from "../../api/contracts";
import { formatOptionalDateTime, formatOptionalDay } from "../../presentation/dates";
import {
  accountHeaderTitle,
  accountIdentityLine,
  formatAccountMoney,
} from "../../presentation/accountLabels";
import { CreditUsageMeter } from "./CreditUsageMeter";
import { ClosingDayField } from "./ClosingDayField";

/**
 * The account detail page's opening block: which account this is, how much
 * it holds, and when that was last refreshed — flat text and spacing, no
 * card wrapping the whole thing. The balance is the one hero figure; the
 * legal institution name (when different from the short one shown as the
 * title) rides along as a tooltip instead of dominating the header, and the
 * provider's own plumbing (MeuPluggy or otherwise) never surfaces here — see
 * Account.institution_name.
 */
export function AccountHeader({
  account,
  onSaveClosingDay,
  savingClosingDay,
}: {
  account: Account;
  onSaveClosingDay: (closingDay: number | null) => Promise<unknown>;
  savingClosingDay: boolean;
}) {
  const isCredit = account.account_type === "CREDIT";
  const title = accountHeaderTitle(account);
  const legalName =
    account.institution_name !== null && account.institution_name !== title
      ? account.institution_name
      : undefined;
  const identityLine = accountIdentityLine(account);

  return (
    <section className="account-identity" aria-label="Identificação da conta">
      <div className="account-identity-heading">
        <Tooltip title={legalName}>
          <h1 className="account-identity-name">{title}</h1>
        </Tooltip>
        {identityLine !== "" && <span className="account-identity-meta">{identityLine}</span>}
      </div>

      <div className="account-balance">
        <span className="account-balance-label">{isCredit ? "Total em aberto" : "Saldo disponível"}</span>
        <p className="account-balance-value">{formatAccountMoney(account.balance, account.currency_code)}</p>
        {isCredit && <CreditUsageMeter ratio={account.credit_usage_ratio} />}
      </div>

      {isCredit && (
        <ul className="debt-header-legend-inline">
          <li>
            <span>Limite total</span>
            <strong>{formatAccountMoney(account.credit_limit, account.currency_code)}</strong>
          </li>
          <li>
            <span>Limite disponível</span>
            <strong>{formatAccountMoney(account.available_credit_limit, account.currency_code)}</strong>
          </li>
          <ClosingDayField account={account} onSave={onSaveClosingDay} saving={savingClosingDay} />
          <li>
            <span>Vencimento</span>
            <strong>{formatOptionalDay(account.balance_due_date)}</strong>
          </li>
        </ul>
      )}

      <p className="account-updated-at">
        Atualizado em {formatOptionalDateTime(account.provider_updated_at ?? account.updated_at)}
      </p>
    </section>
  );
}
