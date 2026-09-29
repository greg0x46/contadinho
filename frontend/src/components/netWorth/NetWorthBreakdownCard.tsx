import type { NetWorthSnapshot } from "../../api/contracts";
import { formatBRL, formatSignedBRL } from "../../presentation/money";

// A debt payable and a card's current bill-cycle total are summed as
// distinct liabilities, never deduplicated — see internal/networth's
// Breakdown doc comment for why.
//
// Receivables (money owed to the user) have no field here at all: they're a
// future possibility, not a realized asset — see internal/networth's
// Breakdown doc comment.
//
// "Patrimônio líquido" is the page's hero figure — cash, investments, card
// and debts are secondary breakdown figures beside it, same shape as
// AccountsSummary.
export function NetWorthBreakdownCard({ snapshot }: { snapshot: NetWorthSnapshot }) {
  return (
    <section className="net-worth-summary" aria-label="Composição do patrimônio">
      <div className="net-worth-summary-hero">
        <span className="net-worth-summary-hero-label">Patrimônio líquido</span>
        <p className="net-worth-summary-hero-value">{formatBRL(snapshot.net_worth)}</p>
      </div>
      <div className="net-worth-summary-secondary">
        <div className="net-worth-summary-figure">
          <span className="net-worth-summary-figure-label">Caixa</span>
          <span className="net-worth-summary-figure-value">{formatBRL(snapshot.cash_balance)}</span>
        </div>
        <div className="net-worth-summary-figure">
          <span className="net-worth-summary-figure-label">Investimentos</span>
          <span className="net-worth-summary-figure-value">{formatBRL(snapshot.investment_balance)}</span>
        </div>
        <div className="net-worth-summary-figure">
          <span className="net-worth-summary-figure-label">Cartão de crédito</span>
          <span className="net-worth-summary-figure-value">
            {formatSignedBRL(snapshot.credit_card_balance, "outflow")}
          </span>
        </div>
        <div className="net-worth-summary-figure">
          <span className="net-worth-summary-figure-label">Dívidas</span>
          <span className="net-worth-summary-figure-value">
            {formatSignedBRL(snapshot.payables_debt, "outflow")}
          </span>
        </div>
      </div>
    </section>
  );
}
