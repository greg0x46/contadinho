import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Popconfirm, Tag } from "antd";

import type { ScenarioTransaction } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import {
  scenarioTransactionStatusColor,
  scenarioTransactionStatusDashed,
  scenarioTransactionStatusLabel,
} from "../../presentation/scenarioLabels";

function formatProjectedDate(value: string): string {
  const [year, month, day] = value.split("-");
  return `${day}/${month}/${year}`;
}

export function DebtPlanTable({
  installments,
  deletingId,
  onDelete,
  onAllocate,
}: {
  installments: ScenarioTransaction[];
  deletingId: string | null;
  onDelete: (installment: ScenarioTransaction) => void;
  onAllocate: (installment: ScenarioTransaction) => void;
}) {
  const columns: ProColumns<ScenarioTransaction>[] = [
    {
      title: "Data prevista",
      dataIndex: "projected_at",
      render: (_, row) => formatProjectedDate(row.projected_at),
    },
    { title: "Descrição", dataIndex: "description" },
    {
      title: "Valor previsto",
      dataIndex: "amount",
      render: (_, row) => formatBRL(row.amount),
    },
    {
      title: "Situação",
      dataIndex: "status",
      render: (_, row) => (
        <Tag
          color={scenarioTransactionStatusColor[row.status]}
          style={scenarioTransactionStatusDashed[row.status] ? { borderStyle: "dashed" } : undefined}
        >
          {scenarioTransactionStatusLabel[row.status]}
        </Tag>
      ),
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, row) => (
        <>
          <Button type="link" onClick={() => onAllocate(row)}>
            Alocar transação
          </Button>
          <Popconfirm
            title="Excluir parcela"
            description="A parcela planejada será removida do plano."
            onConfirm={() => onDelete(row)}
            okText="Excluir"
            cancelText="Cancelar"
          >
            <Button type="link" danger loading={deletingId === row.id}>
              Excluir
            </Button>
          </Popconfirm>
        </>
      ),
    },
  ];

  return (
    <ProTable<ScenarioTransaction>
      aria-label="Parcelas planejadas"
      columns={columns}
      dataSource={installments}
      rowKey="id"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
      locale={{ emptyText: "Nenhuma parcela planejada ainda." }}
    />
  );
}
