import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Popconfirm } from "antd";

import type { ScenarioTransaction } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";

function formatProjectedDate(value: string): string {
  const [year, month, day] = value.split("-");
  return `${day}/${month}/${year}`;
}

export function DebtPlanTable({
  installments,
  deletingId,
  onDelete,
}: {
  installments: ScenarioTransaction[];
  deletingId: string | null;
  onDelete: (installment: ScenarioTransaction) => void;
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
      title: "Ação",
      valueType: "option",
      render: (_, row) => (
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
