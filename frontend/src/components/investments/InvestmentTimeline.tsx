import { Card, Flex, Tag, Timeline, Typography } from "antd";

import type { InvestmentTransaction } from "../../api/contracts";
import { UnavailableState } from "../AsyncState";
import { formatOptionalDate } from "../../presentation/dates";
import { movementTypeLabel } from "../../presentation/investmentLabels";
import { formatBRL } from "../../presentation/money";

const movementDotColor: Record<string, string> = {
  BUY: "green",
  APPLICATION: "green",
  SELL: "red",
  REDEMPTION: "red",
  DIVIDEND: "blue",
  INCOME: "blue",
};

export function InvestmentTimeline({
  transactions,
  isLoading,
  error,
  onRetry,
}: {
  transactions: InvestmentTransaction[];
  isLoading: boolean;
  error: unknown;
  onRetry: () => void;
}) {
  return (
    <Card title="Movimentações">
      {isLoading && <Typography.Text type="secondary">Carregando movimentações…</Typography.Text>}
      {!isLoading && error !== null && error !== undefined && (
        <UnavailableState onRetry={onRetry}>
          Não foi possível carregar as movimentações deste investimento.
        </UnavailableState>
      )}
      {!isLoading && (error === null || error === undefined) && transactions.length === 0 && (
        <Typography.Text type="secondary">Nenhuma movimentação sincronizada ainda.</Typography.Text>
      )}
      {!isLoading && (error === null || error === undefined) && transactions.length > 0 && (
        <Timeline
          items={transactions.map((transaction) => ({
            color: movementDotColor[transaction.movement_type ?? ""] ?? "gray",
            children: (
              <Flex justify="space-between" align="center" wrap gap="small">
                <span>
                  <strong>{formatOptionalDate(transaction.occurred_at ?? transaction.trade_date)}</strong> ·{" "}
                  {transaction.quantity !== null ? `${transaction.quantity} cota(s) · ` : ""}
                  {transaction.amount !== null ? formatBRL(transaction.amount) : "—"}
                </span>
                <Tag>{movementTypeLabel(transaction.movement_type)}</Tag>
              </Flex>
            ),
          }))}
        />
      )}
    </Card>
  );
}
