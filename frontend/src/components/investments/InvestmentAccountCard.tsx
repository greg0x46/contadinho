import { Alert, Button, Card, Collapse, Popconfirm, Space, Tag } from "antd";

import type {
  InvestmentAccount,
  InvestmentOperation,
  InvestmentPortfolio,
  InvestmentPosition,
  InvestmentSummaryAccount,
} from "../../api/contracts";
import { investmentAccountKindLabel } from "../../presentation/investmentWorkspaceLabels";
import { formatBRL, sumBRL } from "../../presentation/money";
import { InvestmentFigures } from "./InvestmentFigures";
import { InvestmentOperationsTable } from "./InvestmentOperationsTable";
import { InvestmentPositionsTable } from "./InvestmentPositionsTable";
import { accountMovements, formatDate, latestDate } from "./investmentFigures";

export function InvestmentAccountCard({
  account,
  summary,
  positions,
  allPositions,
  operations,
  portfolios,
  onRename,
  onRemove,
  onNewPosition,
  onNewOperation,
  onEditPosition,
  onRemovePosition,
  onEditOperation,
  onRemoveOperation,
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
  onRename: (account: InvestmentAccount) => void;
  onRemove: (account: InvestmentAccount) => void;
  onNewPosition: (account: InvestmentAccount) => void;
  onNewOperation: (account: InvestmentAccount) => void;
  onEditPosition: (position: InvestmentPosition) => void;
  onRemovePosition: (position: InvestmentPosition) => void;
  onEditOperation: (operation: InvestmentOperation) => void;
  onRemoveOperation: (operation: InvestmentOperation) => void;
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

  return (
    <Card
      title={
        <Space>
          <span>{account.name}</span>
          <Tag color={editable ? "default" : "blue"}>{investmentAccountKindLabel[account.kind]}</Tag>
          {!account.active && <Tag>Inativa</Tag>}
        </Space>
      }
      style={{ marginBottom: 16 }}
      extra={
        editable ? (
          <Space wrap>
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
          </Space>
        ) : <Space>
          <Button size="small" onClick={() => onNewOperation(account)}>Registrar movimentação</Button>
          <Button size="small" onClick={() => onRename(account)}>
            {account.financial_account_id === null ? "Vincular caixa da corretora" : "Caixa da corretora"}
          </Button>
        </Space>
      }
    >
      {!editable && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="Conta integrada"
          description="Saldos e posições vêm da instituição. Registre aportes, resgates e rendimentos para conciliá-los com o extrato, sem alterar o saldo informado."
        />
      )}
      <InvestmentFigures
        figures={[
          { label: "Valor atual", value: formatBRL(currentValue) },
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
        onAssignGoal={onAssignGoal}
        onEdit={editable ? onEditPosition : undefined}
        onDelete={editable ? onRemovePosition : undefined}
        busy={busy}
        emptyText="Nenhuma posição nesta conta."
      />

      <Collapse
        ghost
        items={[
          {
            key: "operations",
            label: `Movimentações (${operations.length})`,
            children: (
              <InvestmentOperationsTable
                operations={operations}
                positions={allPositions}
                onEdit={onEditOperation}
                onDelete={onRemoveOperation}
                busy={busy}
              />
            ),
          },
        ]}
      />
    </Card>
  );
}
