import { Alert } from "antd";

import type { InvestmentOperation, InvestmentSummary } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import { accountMovements } from "./investmentFigures";

/**
 * The page's hero figure — total value across every custody account — with
 * aportes, resgates, caixa disponível and o quanto vem de instituições como
 * secondary figures beside it. Same shape as AccountsSummary.
 */
export function InvestmentWorkspaceSummary({
  summary,
  operations,
}: {
  summary: InvestmentSummary | null;
  operations: InvestmentOperation[];
}) {
  const movements = accountMovements(operations);

  return (
    <>
      <section className="investments-summary" aria-label="Total investido">
        <div className="investments-summary-hero">
          <span className="investments-summary-hero-label">Valor atual</span>
          <p className="investments-summary-hero-value">{formatBRL(summary?.total_value ?? "0")}</p>
        </div>
        <div className="investments-summary-secondary">
          <div className="investments-summary-figure">
            <span className="investments-summary-figure-label">Aportes</span>
            <span className="investments-summary-figure-value">{formatBRL(movements.deposits)}</span>
          </div>
          <div className="investments-summary-figure">
            <span className="investments-summary-figure-label">Resgates</span>
            <span className="investments-summary-figure-value">{formatBRL(movements.withdrawals)}</span>
          </div>
          <div className="investments-summary-figure">
            <span
              className="investments-summary-figure-label"
              title="Já saiu da conta corrente, ainda não foi aplicado em nenhuma posição."
            >
              Caixa disponível para investir
            </span>
            <span className="investments-summary-figure-value">{formatBRL(summary?.cash_balance ?? "0")}</span>
          </div>
          <div className="investments-summary-figure">
            <span
              className="investments-summary-figure-label"
              title="Parte do total que vem da integração bancária; o restante é registro manual."
            >
              Informado pela instituição
            </span>
            <span className="investments-summary-figure-value">{formatBRL(summary?.synced_value ?? "0")}</span>
          </div>
        </div>
      </section>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="Objetivos apenas agrupam valor"
        description="Um objetivo mostra quanto das suas posições está reservado para ele. Ele não soma nada ao total investido nem ao patrimônio, e mudar o objetivo de uma posição não move dinheiro."
      />
    </>
  );
}
