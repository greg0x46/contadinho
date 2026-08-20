import { WalletOutlined } from "@ant-design/icons";

import type { Account } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import { WidgetCard } from "../shared/WidgetCard";

// Totals cover BRL accounts only: mixing currencies into one figure would be
// wrong, and the same restriction already applies to the credit-card total on
// the home dashboard.
function sumBRL(accounts: Account[], pick: (account: Account) => string | null): string {
  const total = accounts
    .filter((account) => account.currency_code === null || account.currency_code === "BRL")
    .reduce((running, account) => running + Number(pick(account) ?? "0"), 0);
  return total.toFixed(2);
}

export function AccountsSummary({ accounts }: { accounts: Account[] }) {
  const bank = accounts.filter((account) => account.account_type !== "CREDIT");
  const credit = accounts.filter((account) => account.account_type === "CREDIT");

  return (
    <WidgetCard
      icon={<WalletOutlined aria-hidden="true" />}
      title="Resumo das contas"
      style={{ marginBottom: 16 }}
    >
      <div className="debts-summary-body">
        <div className="debts-summary-figure">
          <p className="dashboard-hero-figure">
            {formatBRL(sumBRL(bank, (account) => account.balance))}
          </p>
          <p className="debts-summary-caption">
            em {bank.length} conta(s) bancária(s), somando apenas valores em reais
          </p>
        </div>
        <div className="debts-summary-counts">
          <div className="debts-summary-chip">
            <span className="debts-summary-chip-label">Devido em cartões</span>
            <span className="debts-summary-chip-value">
              {formatBRL(sumBRL(credit, (account) => account.balance))}
            </span>
          </div>
          <div className="debts-summary-chip">
            <span className="debts-summary-chip-label">Limite total</span>
            <span className="debts-summary-chip-value">
              {formatBRL(sumBRL(credit, (account) => account.credit_limit))}
            </span>
          </div>
          <div className="debts-summary-chip">
            <span className="debts-summary-chip-label">Cartões</span>
            <span className="debts-summary-chip-value">{credit.length}</span>
          </div>
        </div>
      </div>
    </WidgetCard>
  );
}
