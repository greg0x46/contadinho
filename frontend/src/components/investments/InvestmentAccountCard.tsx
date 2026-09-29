import { Button, Popconfirm, Tag, Tooltip } from "antd";

import type {
  InvestmentAccount,
  InvestmentOperation,
  InvestmentPortfolio,
  InvestmentPosition,
  InvestmentSummaryAccount,
} from "../../api/contracts";
import { investmentAccountKindLabel } from "../../presentation/investmentWorkspaceLabels";
import { formatBRL, sumBRL } from "../../presentation/money";
import { InvestmentFigures, InvestmentYieldValue } from "./InvestmentFigures";
import { InvestmentPositionsTable } from "./InvestmentPositionsTable";
import {
  accountMovements,
  aggregateYield,
  costsByPosition,
  formatDate,
  groupCost,
  latestDate,
  type LinkedInvestments,
} from "./investmentFigures";

export function InvestmentAccountCard({
  account,
  summary,
  positions,
  allPositions,
  operations,
  portfolios,
  linked,
  onRename,
  onRemove,
  onNewPosition,
  onNewOperation,
  onEditPosition,
  onRemovePosition,
  onAssignGoal,
  busy,
}: {
  account: InvestmentAccount;
  summary: InvestmentSummaryAccount | undefined;
  /** Rows of the positions table (closed ones hidden unless asked for). */
  positions: InvestmentPosition[];
  /** Every position of the account, so operations of a closed one keep its name. */
  allPositions: InvestmentPosition[];
  operations: InvestmentOperation[];
  portfolios: InvestmentPortfolio[];
  linked: LinkedInvestments;
  onRename: (account: InvestmentAccount) => void;
  onRemove: (account: InvestmentAccount) => void;
  onNewPosition: (account: InvestmentAccount) => void;
  onNewOperation: (account: InvestmentAccount) => void;
  onEditPosition: (position: InvestmentPosition) => void;
  onRemovePosition: (position: InvestmentPosition) => void;
  onAssignGoal: (position: InvestmentPosition, portfolioId: string | null) => void;
  busy: boolean;
}) {
  const editable = account.kind === "manual";
  const movements = accountMovements(operations);
  // The server already nets the account's value; positions alone would miss
  // the cash sitting in the custody account.
  const currentValue = sumBRL([summary?.current_value ?? sumBRL(positions.filter((position) => position.currency_code === "BRL").map((position) => position.current_value)), summary?.cash_balance ?? account.cash_balance]);
  const updatedOn = latestDate([
    account.updated_at,
    ...positions.map((position) => position.valued_on),
    ...operations.map((operation) => operation.occurred_on),
  ]);

  const costs = costsByPosition(operations, allPositions, linked);
  const yieldAggregate = aggregateYield(positions, linked, costs);
  const cost = groupCost(operations, allPositions, linked);

  return (
    <section className="investment-card" aria-label={account.name}>
      <header className="investment-card-header">
        <div className="investment-card-identity">
          <h2 className="investment-card-name">{account.name}</h2>
          {editable ? (
            <Tag>{investmentAccountKindLabel[account.kind]}</Tag>
          ) : (
            // What "integrada" means is one hover/tap away, not a permanent alert.
            <Tooltip title="Saldos e posições vêm da instituição. Registre aportes, resgates e rendimentos para conciliá-los com o extrato, sem alterar o saldo informado.">
              <Tag color="blue" tabIndex={0}>
                {investmentAccountKindLabel[account.kind]}
              </Tag>
            </Tooltip>
          )}
          {!account.active && <Tag>Inativa</Tag>}
        </div>
        <div className="investment-card-actions">
          {editable ? (
            <>
              <Button size="small" onClick={() => onNewOperation(account)}>
                Registrar movimentação
              </Button>
              <Button size="small" onClick={() => onNewPosition(account)}>
                Nova posição
              </Button>
              <Button size="small" onClick={() => onRename(account)}>
                Editar conta
              </Button>
              <Popconfirm
                title="Remover conta de custódia"
                description="Só é possível remover uma conta sem posições nem movimentações."
                okText="Remover"
                cancelText="Cancelar"
                okButtonProps={{ danger: true }}
                onConfirm={() => onRemove(account)}
              >
                <Button size="small" danger disabled={busy}>
                  Remover conta
                </Button>
              </Popconfirm>
            </>
          ) : (
            <>
              <Button size="small" onClick={() => onNewOperation(account)}>
                Registrar movimentação
              </Button>
              <Button size="small" onClick={() => onRename(account)}>
                {account.financial_account_id === null ? "Vincular caixa da corretora" : "Caixa da corretora"}
              </Button>
            </>
          )}
        </div>
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
              label: "Aportes",
              value: formatBRL(movements.deposits),
              hint: "Dinheiro que entrou nesta conta de custódia vindo do seu caixa.",
            },
            {
              label: "Resgates",
              value: formatBRL(movements.withdrawals),
              hint: "Dinheiro que saiu desta conta de custódia de volta para o seu caixa.",
            },
            {
              label: "Caixa disponível para investir",
              value: formatBRL(summary?.cash_balance ?? account.cash_balance),
              hint: "Valor já aportado que ainda não foi aplicado em nenhuma posição.",
            },
            { label: "Última atualização", value: formatDate(updatedOn) },
          ]}
        />

        <InvestmentPositionsTable
          positions={positions}
          portfolios={portfolios}
          costs={costs}
          linked={linked}
          onAssignGoal={onAssignGoal}
          onEdit={editable ? onEditPosition : undefined}
          onDelete={editable ? onRemovePosition : undefined}
          busy={busy}
          emptyText="Nenhuma posição nesta conta."
        />
      </div>
    </section>
  );
}
