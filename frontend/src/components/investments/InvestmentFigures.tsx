import { Flex, Tooltip, Typography } from "antd";
import type { ReactNode } from "react";

export type InvestmentFigure = { label: string; value: ReactNode; hint?: string };

/**
 * The account and the goal views answer the same five questions, so they share
 * one row instead of drifting into two slightly different vocabularies.
 */
export function InvestmentFigures({ figures }: { figures: InvestmentFigure[] }) {
  return (
    <Flex gap="large" wrap style={{ marginBottom: 12 }}>
      {figures.map((figure) => (
        <Flex key={figure.label} vertical gap={0} style={{ minWidth: 150 }}>
          <Typography.Text type="secondary">
            {figure.hint ? (
              <Tooltip title={figure.hint}>
                <span>{figure.label}</span>
              </Tooltip>
            ) : (
              figure.label
            )}
          </Typography.Text>
          <strong style={{ fontVariantNumeric: "tabular-nums" }}>{figure.value}</strong>
        </Flex>
      ))}
    </Flex>
  );
}
