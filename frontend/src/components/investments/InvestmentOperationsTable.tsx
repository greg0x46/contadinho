import { Button, Popconfirm, Space, Table, Tag, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";

import type { InvestmentOperation, InvestmentPosition } from "../../api/contracts";
import { investmentOperationKindLabel } from "../../presentation/investmentWorkspaceLabels";
import { formatBRL } from "../../presentation/money";
import { formatDate } from "./investmentFigures";

export function InvestmentOperationsTable({
  operations,
  positions,
  onEdit,
  onDelete,
  busy,
}: {
  operations: InvestmentOperation[];
  positions: InvestmentPosition[];
  onEdit: (operation: InvestmentOperation) => void;
  onDelete: (operation: InvestmentOperation) => void;
  busy: boolean;
}) {
  const positionName = new Map(positions.map((position) => [position.id, position.name]));

  const columns: ColumnsType<InvestmentOperation> = [
    { title: "Data", key: "occurred_on", render: (_, operation) => formatDate(operation.occurred_on) },
    {
      title: "Tipo",
      key: "kind",
      render: (_, operation) => (
        <Space size={4}>
          <span>{investmentOperationKindLabel[operation.kind]}</span>
          {operation.source === "synced" && <Tag color="blue">Informado pela instituição</Tag>}
        </Space>
      ),
    },
    {
      title: "Posição",
      key: "position",
      render: (_, operation) =>
        operation.position_id ? positionName.get(operation.position_id) ?? "Posição removida" : "Só caixa",
    },
    {
      title: "Valor",
      key: "amount",
      render: (_, operation) => (
        <span style={{ fontVariantNumeric: "tabular-nums" }}>{formatBRL(operation.amount)}</span>
      ),
    },
    {
      title: "Ações",
      key: "actions",
      render: (_, operation) =>
        operation.is_editable ? (
          <Space>
            <Button size="small" disabled={busy} onClick={() => onEdit(operation)}>
              Corrigir
            </Button>
            <Popconfirm
              title="Excluir movimentação"
              description="Os saldos e custos da conta serão recalculados."
              okText="Excluir"
              cancelText="Cancelar"
              okButtonProps={{ danger: true }}
              onConfirm={() => onDelete(operation)}
            >
              <Button size="small" danger disabled={busy}>
                Excluir
              </Button>
            </Popconfirm>
          </Space>
        ) : (
          <Typography.Text type="secondary">Somente leitura</Typography.Text>
        ),
    },
  ];

  return (
    <Table<InvestmentOperation>
      aria-label="Movimentações"
      size="small"
      rowKey="id"
      columns={columns}
      dataSource={operations}
      pagination={false}
      locale={{ emptyText: "Nenhuma movimentação registrada nesta conta." }}
      scroll={{ x: "max-content" }}
    />
  );
}
