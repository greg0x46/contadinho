import type { Payable, PayableKind } from "../../api/contracts";
import { payableVocabulary } from "../../presentation/payableLabels";
import { formatBRL } from "../../presentation/money";
import { WidgetCard } from "../shared/WidgetCard";

function sum(values: string[]): number {
  return values.reduce((total, value) => total + Number(value), 0);
}

export function PayablesSummary({ kind, payables }: { kind: PayableKind; payables: Payable[] }) {
  const vocab = payableVocabulary[kind];
  const open = payables.filter((payable) => payable.status === "open");
  const settled = payables.filter((payable) => payable.status === "settled");
  const remainingTotal = sum(open.map((payable) => payable.remaining_amount));

  return (
    <WidgetCard icon={vocab.icon} title={vocab.summaryTitle} style={{ marginBottom: 16 }}>
      <div className="debts-summary-body">
        <div className="debts-summary-figure">
          <p className="dashboard-hero-figure">{formatBRL(remainingTotal.toFixed(2))}</p>
          <p className="debts-summary-caption">{vocab.summaryCaption}</p>
        </div>
        <div className="debts-summary-counts">
          <div className="debts-summary-chip">
            <span className="debts-summary-chip-label">Abertas</span>
            <span className="debts-summary-chip-value">{open.length}</span>
          </div>
          <div className="debts-summary-chip">
            <span className="debts-summary-chip-label">{vocab.settledCountLabel}</span>
            <span className="debts-summary-chip-value">{settled.length}</span>
          </div>
        </div>
      </div>
    </WidgetCard>
  );
}
