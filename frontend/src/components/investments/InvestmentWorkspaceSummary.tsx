import type { InvestmentOperation, InvestmentPosition, InvestmentSummary } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import { InvestmentYieldValue } from "./InvestmentFigures";
import { accountMovements, aggregateYield, costsByPosition, groupCost, type LinkedInvestments } from "./investmentFigures";

/**
 * The page's hero figure — total value across every custody account — with
 * aportes, resgates, caixa disponível and o quanto vem de instituições como
 * secondary figures beside it. Same shape as AccountsSummary.
 */
export function InvestmentWorkspaceSummary({
  summary,
  positions,
  operations,
  linked,
}: {
  summary: InvestmentSummary | null;
  positions: InvestmentPosition[];
  operations: InvestmentOperation[];
  linked: LinkedInvestments;
}) {
  const movements = accountMovements(operations);
  const yieldAggregate = aggregateYield(positions, linked, costsByPosition(operations, positions, linked));
  const cost = groupCost(operations, positions, linked);

  return (
    <section className="investments-summary" aria-label="Total investido">
      <div className="investments-summary-hero">
        <span className="investments-summary-hero-label">Valor atual</span>
        <p className="investments-summary-hero-value">{formatBRL(summary?.total_value ?? "0")}</p>
      </div>
      <div className="investments-summary-secondary">
        <div className="investments-summary-figure">
          <span
            className="investments-summary-figure-label"
            title="Soma do rendimento das posições que o têm, já descontados IR, IOF e taxas."
          >
            Rendimento líquido
          </span>
          <span className="investments-summary-figure-value">
            <InvestmentYieldValue gain={yieldAggregate.gain} percent={yieldAggregate.percent} />
          </span>
        </div>
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
            title="Taxas e impostos das movimentações, mais IR e IOF informados pela instituição."
          >
            IR/Taxas
          </span>
          <span className="investments-summary-figure-value">{formatBRL(cost)}</span>
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
  );
}
