import { Button, Card, Popconfirm, Progress, Space, Tag, Typography } from "antd";

import type {
  InvestmentOperation,
  InvestmentPortfolio,
  InvestmentPosition,
  InvestmentSummaryPortfolio,
} from "../../api/contracts";
import { formatBRL, sumBRL } from "../../presentation/money";
import { InvestmentFigures } from "./InvestmentFigures";
import { InvestmentPositionsTable } from "./InvestmentPositionsTable";
import { formatDate, goalMovements, latestDate } from "./investmentFigures";

export function InvestmentGoalCard({
  portfolio,
  summary,
  positions,
  operations,
  portfolios,
  cashBalance,
  accountNameOf,
  onEdit,
  onRemove,
  onAssignGoal,
  busy,
}: {
  /** null is the "sem objetivo" bucket: positions nobody has grouped yet. */
  portfolio: InvestmentPortfolio | null;
  summary: InvestmentSummaryPortfolio | undefined;
  positions: InvestmentPosition[];
  operations: InvestmentOperation[];
  portfolios: InvestmentPortfolio[];
  cashBalance: string;
  accountNameOf: (accountId: string) => string;
  onEdit: (portfolio: InvestmentPortfolio) => void;
  onRemove: (portfolio: InvestmentPortfolio) => void;
  onAssignGoal: (position: InvestmentPosition, portfolioId: string | null) => void;
  busy: boolean;
}) {
  const movements = goalMovements(operations);
  const currentValue =
    summary?.current_value ?? portfolio?.current_value ?? sumBRL(positions.filter((position) => position.currency_code === "BRL").map((position) => position.current_value));
  const updatedOn = latestDate([
    ...positions.map((position) => position.valued_on),
    ...operations.map((operation) => operation.occurred_on),
  ]);
  const target = summary?.target_amount ?? portfolio?.target_amount ?? null;
  const progress = summary?.progress ?? portfolio?.progress ?? null;

  return (
    <Card
      title={
        <Space>
          <span>{portfolio?.name ?? "Sem objetivo"}</span>
          <Tag>Agrupamento</Tag>
        </Space>
      }
      style={{ marginBottom: 16 }}
      extra={
        portfolio ? (
          <Space>
            <Button size="small" onClick={() => onEdit(portfolio)}>
              Editar objetivo
            </Button>
            <Popconfirm
              title="Remover objetivo"
              description="As posições continuam onde estão; elas apenas deixam de ter objetivo."
              okText="Remover"
              cancelText="Cancelar"
              okButtonProps={{ danger: true }}
              onConfirm={() => onRemove(portfolio)}
            >
              <Button size="small" danger disabled={busy}>
                Remover objetivo
              </Button>
            </Popconfirm>
          </Space>
        ) : null
      }
    >
      <InvestmentFigures
        figures={[
          { label: "Valor atual", value: formatBRL(currentValue) },
          {
            label: "Compras e saldo inicial",
            value: formatBRL(movements.deposits),
            hint: "Compras e saldos iniciais registrados nas posições deste objetivo.",
          },
          {
            label: "Vendas",
            value: formatBRL(movements.withdrawals),
            hint: "Vendas registradas nas posições deste objetivo.",
          },
          {
            label: "Caixa disponível para investir",
            value: formatBRL(cashBalance),
            hint: "O caixa fica na conta de custódia e não pertence a nenhum objetivo; este é o total de todas as contas.",
          },
          { label: "Última atualização", value: formatDate(updatedOn) },
        ]}
      />

      {target !== null && (
        <Space direction="vertical" size={0} style={{ marginBottom: 12, width: "100%" }}>
          <Typography.Text type="secondary">Meta de {formatBRL(target)}</Typography.Text>
          {progress !== null && <Progress percent={Math.min(100, Math.round(Number(progress) * 100))} />}
        </Space>
      )}

      <InvestmentPositionsTable
        positions={positions}
        portfolios={portfolios}
        accountNameOf={accountNameOf}
        showAccount
        onAssignGoal={onAssignGoal}
        busy={busy}
        emptyText={
          portfolio
            ? "Nenhuma posição neste objetivo ainda. Escolha o objetivo na coluna “Objetivo” de qualquer posição."
            : "Todas as posições já têm um objetivo."
        }
      />
    </Card>
  );
}
