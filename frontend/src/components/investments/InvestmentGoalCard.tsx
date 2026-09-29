import { Button, Popconfirm, Progress, Tag } from "antd";

import type {
  InvestmentOperation,
  InvestmentPortfolio,
  InvestmentPosition,
  InvestmentSummaryPortfolio,
} from "../../api/contracts";
import { formatBRL, sumBRL } from "../../presentation/money";
import { InvestmentFigures, InvestmentYieldValue } from "./InvestmentFigures";
import { InvestmentPositionsTable } from "./InvestmentPositionsTable";
import {
  aggregateYield,
  costsByPosition,
  formatDate,
  goalMovements,
  groupCost,
  latestDate,
  type LinkedInvestments,
} from "./investmentFigures";

export function InvestmentGoalCard({
  portfolio,
  summary,
  positions,
  operations,
  portfolios,
  linked,
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
  linked: LinkedInvestments;
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

  const costs = costsByPosition(operations, positions, linked);
  const yieldAggregate = aggregateYield(positions, linked, costs);
  const cost = groupCost(operations, positions, linked);

  return (
    <section className="investment-card" aria-label={portfolio?.name ?? "Sem objetivo"}>
      <header className="investment-card-header">
        <div className="investment-card-identity">
          <h2 className="investment-card-name">{portfolio?.name ?? "Sem objetivo"}</h2>
          <Tag>Agrupamento</Tag>
        </div>
        {portfolio && (
          <div className="investment-card-actions">
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
          </div>
        )}
      </header>

      <div className="investment-card-body">
        <InvestmentFigures
          figures={[
            { label: "Valor atual", value: formatBRL(currentValue) },
            {
              label: "Rendimento líquido",
              value: <InvestmentYieldValue gain={yieldAggregate.gain} percent={yieldAggregate.percent} />,
              hint: "Já descontados IR, IOF e taxas das posições que têm rendimento conhecido.",
            },
            {
              label: "IR/Taxas",
              value: formatBRL(cost),
              hint: "Taxas e impostos das movimentações, mais IR e IOF informados pela instituição.",
            },
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
          <div className="investment-card-goal">
            <span>Meta de {formatBRL(target)}</span>
            {progress !== null && <Progress percent={Math.min(100, Math.round(Number(progress) * 100))} />}
          </div>
        )}

        <InvestmentPositionsTable
          positions={positions}
          portfolios={portfolios}
          costs={costs}
          linked={linked}
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
      </div>
    </section>
  );
}
