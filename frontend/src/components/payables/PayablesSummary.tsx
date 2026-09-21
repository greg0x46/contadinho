import type { Payable, PayableKind } from "../../api/contracts";
import { payableVocabulary } from "../../presentation/payableLabels";
import { formatBRL } from "../../presentation/money";
import { DataCardSummary } from "../layout/DataCard";

function sum(values: string[]): number {
  return values.reduce((total, value) => total + Number(value), 0);
}

/** The summary strip of the payables DataCard: what is still open, and how many are open or settled. */
export function PayablesSummary({ kind, payables }: { kind: PayableKind; payables: Payable[] }) {
  const vocab = payableVocabulary[kind];
  const open = payables.filter((payable) => payable.status === "open");
  const settled = payables.filter((payable) => payable.status === "settled");
  const remainingTotal = sum(open.map((payable) => payable.remaining_amount));

  return (
    <DataCardSummary label={vocab.summaryTitle}>
      <div className="debts-summary-body">
        <div className="debts-summary-figure">
          <p className="debts-summary-value">
            <span aria-hidden="true">{vocab.icon}</span> {formatBRL(remainingTotal.toFixed(2))}
          </p>
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
    </DataCardSummary>
  );
}
