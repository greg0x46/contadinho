import { WalletOutlined } from "@ant-design/icons";
import { Card } from "antd";

import type { Debt } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";

function sum(values: string[]): number {
  return values.reduce((total, value) => total + Number(value), 0);
}

export function DebtsSummary({ debts }: { debts: Debt[] }) {
  const open = debts.filter((debt) => debt.status === "open");
  const settled = debts.filter((debt) => debt.status === "settled");
  const remainingTotal = sum(open.map((debt) => debt.remaining_amount));

  return (
    <Card
      className="dashboard-widget"
      style={{ marginBottom: 16 }}
      title={
        <span className="dashboard-widget-title">
          <WalletOutlined aria-hidden="true" />
          Resumo das dívidas
        </span>
      }
    >
      <div className="debts-summary-body">
        <div className="debts-summary-figure">
          <p className="dashboard-hero-figure">{formatBRL(remainingTotal.toFixed(2))}</p>
          <p className="debts-summary-caption">restantes em dívidas abertas</p>
        </div>
        <div className="debts-summary-counts">
          <div className="debts-summary-chip">
            <span className="debts-summary-chip-label">Abertas</span>
            <span className="debts-summary-chip-value">{open.length}</span>
          </div>
          <div className="debts-summary-chip">
            <span className="debts-summary-chip-label">Quitadas</span>
            <span className="debts-summary-chip-value">{settled.length}</span>
          </div>
        </div>
      </div>
    </Card>
  );
}
