import type { Account } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import { DataCardSummary } from "../layout/DataCard";

// Totals cover BRL accounts only: mixing currencies into one figure would be
// wrong, and the same restriction already applies to the credit-card total on
// the home dashboard.
function sumBRL(accounts: Account[], pick: (account: Account) => string | null): string {
  const total = accounts
    .filter((account) => account.currency_code === null || account.currency_code === "BRL")
    .reduce((running, account) => running + Number(pick(account) ?? "0"), 0);
  return total.toFixed(2);
}

function countLabel(count: number, singular: string, plural: string): string {
  return `${count} ${count === 1 ? singular : plural}`;
}

/**
 * The page's flat opening summary: saldo total is the one figure with real
 * weight, the invoice/limit/available figures ride along as compact
 * secondary metrics, and how many accounts/cards exist is metadata at the
 * end — never a card of its own.
 */
export function FinancialSummary({ accounts, busy }: { accounts: Account[]; busy?: boolean }) {
  const bank = accounts.filter((account) => account.account_type !== "CREDIT");
  const credit = accounts.filter((account) => account.account_type === "CREDIT");
  const hasCredit = credit.length > 0;

  return (
    <DataCardSummary
      label="Resumo financeiro"
      busy={busy}
      note="Os dados são importados automaticamente da sua instituição financeira."
    >
      <div className="accounts-summary-bar">
        <div className="accounts-summary-total">
          <span className="accounts-summary-total-label">Saldo total</span>
          <span className="accounts-summary-total-value">
            {formatBRL(sumBRL(bank, (account) => account.balance))}
          </span>
        </div>

        {hasCredit && (
          <dl className="accounts-summary-metrics">
            <div className="accounts-summary-metric">
              <dt>Fatura atual</dt>
              <dd>{formatBRL(sumBRL(credit, (account) => account.balance))}</dd>
            </div>
            <div className="accounts-summary-metric">
              <dt>Limite total</dt>
              <dd>{formatBRL(sumBRL(credit, (account) => account.credit_limit))}</dd>
            </div>
            <div className="accounts-summary-metric">
              <dt>Disponível</dt>
              <dd>{formatBRL(sumBRL(credit, (account) => account.available_credit_limit))}</dd>
            </div>
          </dl>
        )}

        <span className="accounts-summary-count">
          {countLabel(bank.length, "conta bancária", "contas bancárias")}
          {hasCredit ? ` · ${countLabel(credit.length, "cartão", "cartões")}` : ""}
        </span>
      </div>
    </DataCardSummary>
  );
}
