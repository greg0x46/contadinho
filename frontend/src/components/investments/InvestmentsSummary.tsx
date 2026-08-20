import { RiseOutlined } from "@ant-design/icons";

import type { Investment } from "../../api/contracts";
import { investmentYield } from "../../presentation/investmentLabels";
import { formatBRL } from "../../presentation/money";
import { WidgetCard } from "../shared/WidgetCard";

function sum(values: string[]): number {
  return values.reduce((total, value) => total + Number(value), 0);
}

export function InvestmentsSummary({ investments }: { investments: Investment[] }) {
  const balanceTotal = sum(investments.map((investment) => investment.balance ?? "0"));
  const yields = investments.map((investment) => investmentYield(investment)).filter((y) => y !== null);
  const yieldTotal = sum(yields.map((y) => y.value));

  return (
    <WidgetCard
      icon={<RiseOutlined aria-hidden="true" />}
      title="Resumo dos investimentos"
      style={{ marginBottom: 16 }}
    >
      <div className="debts-summary-body">
        <div className="debts-summary-figure">
          <p className="dashboard-hero-figure">{formatBRL(balanceTotal.toFixed(2))}</p>
          <p className="debts-summary-caption">aplicados em {investments.length} investimento(s)</p>
        </div>
        <div className="debts-summary-counts">
          <div className="debts-summary-chip">
            <span className="debts-summary-chip-label">Rendimento acumulado</span>
            <span className="debts-summary-chip-value">{formatBRL(yieldTotal.toFixed(2))}</span>
          </div>
          {yields.length < investments.length && (
            <div className="debts-summary-chip">
              <span className="debts-summary-chip-label">Sem dado de rendimento</span>
              <span className="debts-summary-chip-value">{investments.length - yields.length}</span>
            </div>
          )}
        </div>
      </div>
    </WidgetCard>
  );
}
