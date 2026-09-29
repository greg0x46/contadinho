import { Button, Popconfirm, Select, Space, Table, Tag, Tooltip, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { Link } from "react-router-dom";

import type { InvestmentPortfolio, InvestmentPosition } from "../../api/contracts";
import { investmentValuationBasisLabel } from "../../presentation/investmentWorkspaceLabels";
import { formatBRL, formatMoney } from "../../presentation/money";
import { InvestmentYieldValue } from "./InvestmentFigures";
import { formatDate, formatQuantity, isZeroBRL, positionYield, type LinkedInvestments } from "./investmentFigures";

function YieldCell({
  position,
  linked,
  cost,
}: {
  position: InvestmentPosition;
  linked: LinkedInvestments;
  cost: string;
}) {
  const estimate = positionYield(position, linked, cost);
  if (!estimate.known) {
    return (
      <Tooltip title={estimate.reason}>
        <Typography.Text type="secondary">Rentabilidade indisponível</Typography.Text>
      </Tooltip>
    );
  }
  return (
    <Tooltip title={`Líquido de IR, IOF e taxas · bruto ${formatBRL(estimate.gross)}`}>
      <span>
        <InvestmentYieldValue gain={estimate.value} percent={estimate.percent} />
      </span>
    </Tooltip>
  );
}

export function InvestmentPositionsTable({
  positions,
  portfolios,
  costs,
  linked,
  accountNameOf,
  showAccount = false,
  onAssignGoal,
  onEdit,
  onDelete,
  busy,
  emptyText,
}: {
  positions: InvestmentPosition[];
  portfolios: InvestmentPortfolio[];
  /** position_id -> taxas e impostos lançados nas movimentações dessa posição. */
  costs: Record<string, string>;
  linked: LinkedInvestments;
  accountNameOf?: (accountId: string) => string;
  showAccount?: boolean;
  onAssignGoal: (position: InvestmentPosition, portfolioId: string | null) => void;
  onEdit?: (position: InvestmentPosition) => void;
  onDelete?: (position: InvestmentPosition) => void;
  busy: boolean;
  emptyText: string;
}) {
  const goalOptions = portfolios.map((portfolio) => ({ value: portfolio.id, label: portfolio.name }));

  const columns: ColumnsType<InvestmentPosition> = [
    {
      title: "Posição",
      key: "name",
      render: (_, position) => (
        <Space direction="vertical" size={0}>
          <span>
            {position.linked_investment_id ? (
              <Link to={`/investimentos/${position.linked_investment_id}`}>{position.name}</Link>
            ) : (
              position.name
            )}
            {position.closed && <Tag style={{ marginLeft: 8 }}>Encerrada</Tag>}
          </span>
          <Typography.Text type="secondary">
            {[position.ticker, position.asset_type].filter(Boolean).join(" · ")}
          </Typography.Text>
        </Space>
      ),
    },
    ...(showAccount
      ? [
          {
            title: "Conta",
            key: "account",
            render: (_: unknown, position: InvestmentPosition) =>
              accountNameOf?.(position.account_id) ?? "Conta não encontrada",
          },
        ]
      : []),
    {
      title: "Objetivo",
      key: "portfolio",
      render: (_, position) => (
        <Select
          aria-label={`Objetivo de ${position.name}`}
          style={{ minWidth: 200 }}
          value={position.portfolio_id ?? undefined}
          options={goalOptions}
          placeholder="Sem objetivo"
          allowClear
          showSearch
          optionFilterProp="label"
          disabled={busy}
          onChange={(value: string | undefined) => onAssignGoal(position, value ?? null)}
        />
      ),
    },
    {
      title: "Quantidade",
      key: "quantity",
      render: (_, position) => (isZeroBRL(position.quantity) ? "—" : formatQuantity(position.quantity)),
    },
    {
      title: "Custo médio",
      key: "average_cost",
      render: (_, position) =>
        isZeroBRL(position.average_cost) ? "—" : formatBRL(position.average_cost as string),
    },
    {
      title: "Valor atual",
      key: "current_value",
      render: (_, position) => (
        <Space direction="vertical" size={0}>
          <span style={{ fontVariantNumeric: "tabular-nums" }}>{position.currency_code === "BRL" ? formatBRL(position.current_value) : formatMoney(position.current_value, position.currency_code)}</span>
          <Tag color={position.valuation_basis === "provider_balance" ? "blue" : "default"}>
            {investmentValuationBasisLabel[position.valuation_basis]}
          </Tag>
          <Typography.Text type="secondary">
            Atualizado em {formatDate(position.valued_on)}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: "IR/Taxas",
      key: "cost",
      render: (_, position) => (isZeroBRL(costs[position.id] ?? null) ? "—" : formatBRL(costs[position.id])),
    },
    {
      title: "Rendimento líquido",
      key: "yield",
      render: (_, position) => <YieldCell position={position} linked={linked} cost={costs[position.id] ?? "0"} />,
    },
    {
      title: "Ações",
      key: "actions",
      render: (_, position) =>
        position.source === "synced" ? (
          // An imported position is the institution's record: only the goal,
          // which is a local grouping, is ours to change.
          <Typography.Text type="secondary">Somente o objetivo</Typography.Text>
        ) : (
          <Space>
            {onEdit && (
              <Button size="small" onClick={() => onEdit(position)}>
                Editar
              </Button>
            )}
            {onDelete && (
              <Popconfirm
                title="Remover posição"
                description="Só é possível remover uma posição sem movimentações."
                okText="Remover"
                cancelText="Cancelar"
                okButtonProps={{ danger: true }}
                onConfirm={() => onDelete(position)}
              >
                <Button size="small" danger>
                  Remover
                </Button>
              </Popconfirm>
            )}
          </Space>
        ),
    },
  ];

  return (
    <Table<InvestmentPosition>
      aria-label="Posições"
      className="investment-table"
      size="small"
      rowKey="id"
      columns={columns}
      dataSource={positions}
      pagination={false}
      locale={{ emptyText }}
      scroll={{ x: "max-content" }}
    />
  );
}
