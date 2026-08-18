import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Popconfirm, Space, Switch } from "antd";

import type { Category, RecurringCommitment } from "../../api/contracts";
import {
  recurrenceScheduleLabel,
  recurringCommitmentKindLabel,
} from "../../presentation/recurringCommitmentLabels";
import { formatMoney } from "../../presentation/money";

export function RecurringCommitmentList({
  commitments,
  categories,
  isLoading,
  togglingCommitmentId,
  onEdit,
  onToggle,
  onDelete,
}: {
  commitments: RecurringCommitment[];
  categories: Category[];
  isLoading: boolean;
  togglingCommitmentId: string | null;
  onEdit: (commitment: RecurringCommitment) => void;
  onToggle: (commitment: RecurringCommitment, isActive: boolean) => void;
  onDelete: (commitment: RecurringCommitment) => void;
}) {
  const categoryName = (categoryId: string) =>
    categories.find((category) => category.id === categoryId)?.name ?? "Categoria removida";

  const columns: ProColumns<RecurringCommitment>[] = [
    { title: "Nome", dataIndex: "name" },
    {
      title: "Tipo",
      dataIndex: "kind",
      render: (_, commitment) => recurringCommitmentKindLabel[commitment.kind],
    },
    {
      title: "Valor",
      dataIndex: "amount",
      render: (_, commitment) => formatMoney(commitment.amount, "BRL"),
    },
    {
      title: "Categoria",
      dataIndex: "category_id",
      render: (_, commitment) => categoryName(commitment.category_id),
    },
    {
      title: "Recorrência",
      dataIndex: "cadence",
      render: (_, commitment) =>
        recurrenceScheduleLabel(commitment.cadence, commitment.day_of_month, commitment.month_of_year),
    },
    {
      title: "Ativo",
      dataIndex: "is_active",
      render: (_, commitment) => (
        <Switch
          aria-label={`${commitment.is_active ? "Pausar" : "Ativar"} compromisso ${commitment.name}`}
          checked={commitment.is_active}
          loading={togglingCommitmentId === commitment.id}
          onChange={(checked) => onToggle(commitment, checked)}
        />
      ),
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, commitment) => (
        <Space>
          <Button type="link" onClick={() => onEdit(commitment)}>
            Editar
          </Button>
          <Popconfirm
            title="Excluir compromisso"
            description="Esta ação não pode ser desfeita."
            onConfirm={() => onDelete(commitment)}
            okText="Excluir"
            cancelText="Cancelar"
          >
            <Button type="link" danger>
              Excluir
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <ProTable<RecurringCommitment>
      aria-label="Compromissos recorrentes"
      columns={columns}
      dataSource={commitments}
      loading={isLoading}
      rowKey="id"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
      locale={{ emptyText: "Nenhum compromisso recorrente cadastrado ainda." }}
    />
  );
}
