import type { Account } from "../../api/contracts";
import { accountDisplayName, accountMetaParts, formatAccountMoney } from "../../presentation/accountLabels";

/**
 * One bank account: name + saldo carry the weight. Institution, subtype and
 * number ride along as dimmed metadata — stacked under the name on a phone,
 * broken into their own column on wider screens for a tabular scan. The
 * whole row opens the account; the name is the real button, stretched over
 * the row like a transaction row's description.
 */
export function AccountRow({ account, onOpen }: { account: Account; onOpen: (account: Account) => void }) {
  const name = accountDisplayName(account);
  const meta = accountMetaParts(account).join(" · ");

  return (
    <div className="account-row">
      <span className="account-row-identity">
        <button type="button" className="account-row-open" onClick={() => onOpen(account)}>
          {name}
        </button>
        {meta !== "" && <span className="account-row-meta">{meta}</span>}
      </span>
      <span className="account-row-balance">{formatAccountMoney(account.balance, account.currency_code)}</span>
    </div>
  );
}
