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
 * The account detail page's opening block — the one summary for both a bank
 * account and a credit card: which entity this is, how much it holds or
 * owes, and when that was last refreshed. A single lightly-bounded surface
 * (not the old borderless block sitting flush on the page background, and
 * not the heavy all-fields card from before that either) so it reads as one
 * unit at least as structured as the sections below it. Only the content
 * inside adapts to account.account_type; the shape stays the same. The
 * legal institution name (when different from the short one shown as the
 * title) rides along as a tooltip instead of dominating the header, and the
 * provider's own plumbing (MeuPluggy or otherwise) never surfaces here — see
 * Account.institution_name.
 */
export function AccountSummary({
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
    <section className="account-summary" aria-label="Resumo da conta">
      <div className="account-summary-identity">
        <Tooltip title={legalName}>
          <h2 className="account-summary-name">{title}</h2>
        </Tooltip>
        {identityLine !== "" && <span className="account-summary-meta">{identityLine}</span>}
      </div>

      <div className="account-summary-balance">
        <span className="account-summary-balance-label">{isCredit ? "Fatura atual" : "Saldo disponível"}</span>
        <p className="account-summary-balance-value">{formatAccountMoney(account.balance, account.currency_code)}</p>
      </div>

      {isCredit && (
        <>
          <CreditUsageMeter ratio={account.credit_usage_ratio} />

          <div className="account-summary-limits">
            <div className="account-summary-limit is-primary">
              <span>Disponível</span>
              <strong>{formatAccountMoney(account.available_credit_limit, account.currency_code)}</strong>
            </div>
            <div className="account-summary-limit">
              <span>Limite total</span>
              <strong>{formatAccountMoney(account.credit_limit, account.currency_code)}</strong>
            </div>
          </div>

          <div className="account-summary-dates">
            <ClosingDayField account={account} onSave={onSaveClosingDay} saving={savingClosingDay} />
            <div className="account-summary-date">
              <span className="account-summary-date-label">Vence</span>
              <strong className="account-summary-date-value">{formatOptionalDay(account.balance_due_date)}</strong>
            </div>
          </div>
        </>
      )}

      <p className="account-summary-updated">
        Atualizado em {formatOptionalDateTime(account.provider_updated_at ?? account.updated_at)}
      </p>
    </section>
  );
}
