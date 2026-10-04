import { Popover } from "antd";
import type { ReactNode } from "react";

export type InvestmentFigure = { label: string; value: ReactNode; hint?: string };

/**
 * The account and the goal views answer the same five questions, so they share
 * one strip instead of drifting into two slightly different vocabularies. A
 * figure's explanation opens on tap or hover (and from the keyboard) from its
 * underlined label — a hover-only tooltip would be unreachable on a phone.
 */
export function InvestmentFigures({ figures }: { figures: InvestmentFigure[] }) {
  return (
    <dl className="investment-figures">
      {figures.map((figure) => (
        <div key={figure.label} className="investment-figure">
          <dt>
            {figure.hint ? (
              <Popover content={figure.hint} trigger={["hover", "click"]} placement="bottomLeft">
                <button type="button" className="investment-figure-hint">
                  {figure.label}
                </button>
              </Popover>
            ) : (
              figure.label
            )}
          </dt>
          <dd>{figure.value}</dd>
        </div>
      ))}
    </dl>
  );
}
