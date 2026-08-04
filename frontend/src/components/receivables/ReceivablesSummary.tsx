import { DollarOutlined } from "@ant-design/icons";
import { Card } from "antd";

import type { Receivable } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";

function sum(values: string[]): number {
  return values.reduce((total, value) => total + Number(value), 0);
}

export function ReceivablesSummary({ receivables }: { receivables: Receivable[] }) {
  const open = receivables.filter((receivable) => receivable.status === "open");
  const settled = receivables.filter((receivable) => receivable.status === "settled");
  const remainingTotal = sum(open.map((receivable) => receivable.remaining_amount));

  return (
    <Card
      className="dashboard-widget"
      style={{ marginBottom: 16 }}
      title={
        <span className="dashboard-widget-title">
          <DollarOutlined aria-hidden="true" />
          Resumo das contas a receber
        </span>
      }
    >
      <div className="debts-summary-body">
        <div className="debts-summary-figure">
          <p className="dashboard-hero-figure">{formatBRL(remainingTotal.toFixed(2))}</p>
          <p className="debts-summary-caption">a receber em contas abertas</p>
        </div>
        <div className="debts-summary-counts">
          <div className="debts-summary-chip">
            <span className="debts-summary-chip-label">Abertas</span>
            <span className="debts-summary-chip-value">{open.length}</span>
          </div>
          <div className="debts-summary-chip">
            <span className="debts-summary-chip-label">Recebidas</span>
            <span className="debts-summary-chip-value">{settled.length}</span>
          </div>
        </div>
      </div>
    </Card>
  );
}
