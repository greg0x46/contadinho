import type { Account } from "../../api/contracts";
import {
  accountDisplayName,
  creditCardScheduleParts,
  formatAccountMoney,
} from "../../presentation/accountLabels";
import { LimitUsage } from "./LimitUsage";

/**
 * One credit card: name + fatura atual carry the weight. Vencimento and
 * fechamento ride along as a compact schedule line; usage and the amount
 * still available sit at the bottom with their bar. Limite total stays out
 * of the row entirely — it's a click away on the card's own page.
 */
export function CreditCardRow({ account, onOpen }: { account: Account; onOpen: (account: Account) => void }) {
  const schedule = creditCardScheduleParts(account).join(" · ");

  return (
    <div className="credit-card-row">
      <span className="credit-card-row-identity">
        <button type="button" className="credit-card-row-open" onClick={() => onOpen(account)}>
          {accountDisplayName(account)}
        </button>
      </span>

      <span className="credit-card-row-amount-block">
        <span className="credit-card-row-amount">
          {formatAccountMoney(account.balance, account.currency_code)}
        </span>
        <span className="credit-card-row-amount-label">Fatura atual</span>
      </span>

      {schedule !== "" && <span className="credit-card-row-schedule">{schedule}</span>}

      <div className="credit-card-row-usage">
        <LimitUsage
          ratio={account.credit_usage_ratio}
          available={account.available_credit_limit}
          currencyCode={account.currency_code}
        />
      </div>
    </div>
  );
}
