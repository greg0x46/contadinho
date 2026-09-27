import { Tooltip } from "antd";
import type { ReactNode } from "react";

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
