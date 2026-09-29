import { Tooltip } from "antd";
import type { ReactNode } from "react";

import { formatSignedBRL } from "../../presentation/money";
import { formatPercent, isZeroBRL } from "./investmentFigures";

export type InvestmentFigure = { label: string; value: ReactNode; hint?: string };

/**
 * The account and the goal views answer the same five questions, so they share
 * one row instead of drifting into two slightly different vocabularies.
 */
export function InvestmentFigures({ figures }: { figures: InvestmentFigure[] }) {
  return (
    <div className="investment-figures">
      {figures.map((figure) => (
        <div key={figure.label} className="investment-figure">
          <span className="investment-figure-label">
            {figure.hint ? (
              <Tooltip title={figure.hint}>
                <span>{figure.label}</span>
              </Tooltip>
            ) : (
              figure.label
            )}
          </span>
          <span className="investment-figure-value">{figure.value}</span>
        </div>
      ))}
    </div>
  );
}

/**
 * Rendimento in R$ with its % beside it. The sign is spelled out (+/−) so
 * direction never depends on the colour alone.
 */
export function InvestmentYieldValue({ gain, percent }: { gain: string; percent: number | null }) {
  const direction = isZeroBRL(gain) ? "zero" : gain.startsWith("-") ? "negative" : "positive";
  const amount = formatSignedBRL(
    gain,
    direction === "positive" ? "inflow" : direction === "negative" ? "outflow" : "unclassified",
  );
  return (
    <span className={`investment-yield is-${direction}`}>
      <span className="investment-yield-amount">{amount}</span>
      {percent !== null && <span className="investment-yield-percent">{formatPercent(percent)}</span>}
    </span>
  );
}
