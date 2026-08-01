import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Popconfirm, Space, Tag } from "antd";

import type { Debt } from "../../api/contracts";
import { debtStatusColor, debtStatusLabel } from "../../presentation/debtLabels";
import { formatBRL } from "../../presentation/money";

function deleteDescription(debt: Debt): string {
  if (debt.link_count === 0) {
    return "Esta ação não pode ser desfeita.";
  }
  const plural = debt.link_count > 1;
  return `${debt.link_count} transação${plural ? "ões" : ""} vinculada${plural ? "s" : ""} ${
    plural ? "serão desfeitas" : "será desfeita"
  }; as transações em si permanecem inalteradas.`;
}

export function DebtList({
  debts,
  isLoading,
  onOpen,
  onEdit,
  onDelete,
}: {
  debts: Debt[];
  isLoading: boolean;
  onOpen: (debt: Debt) => void;
  onEdit: (debt: Debt) => void;
  onDelete: (debt: Debt) => void;
}) {
  const columns: ProColumns<Debt>[] = [
    { title: "Nome", dataIndex: "name" },
    {
      title: "Valor total",
      dataIndex: "total_amount",
      render: (_, debt) => formatBRL(debt.total_amount),
    },
    {
      title: "Valor pago",
      dataIndex: "paid_amount",
      render: (_, debt) => formatBRL(debt.paid_amount),
    },
    {
      title: "Valor restante",
      dataIndex: "remaining_amount",
      render: (_, debt) => formatBRL(debt.remaining_amount),
    },
    {
      title: "Situação",
      dataIndex: "status",
      render: (_, debt) => (
        <Tag color={debtStatusColor[debt.status]}>{debtStatusLabel[debt.status]}</Tag>
      ),
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, debt) => (
        <Space onClick={(event) => event.stopPropagation()}>
          <Button type="link" onClick={() => onEdit(debt)}>
            Editar
          </Button>
          <Popconfirm
            title="Excluir dívida"
            description={deleteDescription(debt)}
            onConfirm={() => onDelete(debt)}
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
    <ProTable<Debt>
      aria-label="Dívidas"
      columns={columns}
      dataSource={debts}
      loading={isLoading}
      rowKey="id"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
      locale={{ emptyText: "Nenhuma dívida cadastrada ainda." }}
      onRow={(debt) => ({
        onClick: () => onOpen(debt),
        style: { cursor: "pointer" },
      })}
    />
  );
}
