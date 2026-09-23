import type { Account } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";

// Totals cover BRL accounts only: mixing currencies into one figure would be
// wrong, and the same restriction already applies to the credit-card total on
// the home dashboard.
function sumBRL(accounts: Account[], pick: (account: Account) => string | null): string {
  const total = accounts
    .filter((account) => account.currency_code === null || account.currency_code === "BRL")
    .reduce((running, account) => running + Number(pick(account) ?? "0"), 0);
  return total.toFixed(2);
}

/**
 * The page's hero figure — total balance across bank accounts — with the
 * card invoice and available limit as smaller, secondary figures beside it.
 * The credit limit itself isn't repeated here: it belongs to the cards
 * section header, next to the cards it actually limits.
 */
export function AccountsSummary({ accounts }: { accounts: Account[] }) {
  const bank = accounts.filter((account) => account.account_type !== "CREDIT");
  const credit = accounts.filter((account) => account.account_type === "CREDIT");

  return (
    <section className="accounts-summary" aria-label="Resumo das contas">
      <div className="accounts-summary-hero">
        <span className="accounts-summary-hero-label">Saldo total</span>
        <p className="accounts-summary-hero-value">{formatBRL(sumBRL(bank, (account) => account.balance))}</p>
      </div>
      <div className="accounts-summary-secondary">
        <div className="accounts-summary-figure">
          <span className="accounts-summary-figure-label">Fatura dos cartões</span>
          <span className="accounts-summary-figure-value">
            {formatBRL(sumBRL(credit, (account) => account.balance))}
          </span>
        </div>
        <div className="accounts-summary-figure">
          <span className="accounts-summary-figure-label">Limite disponível</span>
          <span className="accounts-summary-figure-value">
            {formatBRL(sumBRL(credit, (account) => account.available_credit_limit))}
          </span>
        </div>
      </div>
    </section>
  );
}
