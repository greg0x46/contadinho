import { DollarOutlined } from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Popconfirm, Space, Tag } from "antd";

import type { Receivable } from "../../api/contracts";
import { receivableStatusColor, receivableStatusLabel } from "../../presentation/receivableLabels";
import { formatBRL } from "../../presentation/money";

function deleteDescription(receivable: Receivable): string {
  if (receivable.link_count === 0) {
    return "Esta ação não pode ser desfeita.";
  }
  const plural = receivable.link_count > 1;
  return `${receivable.link_count} transação${plural ? "ões" : ""} vinculada${plural ? "s" : ""} ${
    plural ? "serão desfeitas" : "será desfeita"
  }; as transações em si permanecem inalteradas.`;
}

function ReceivableProgress({ receivable }: { receivable: Receivable }) {
  const total = Number(receivable.total_amount);
  const received = Number(receivable.received_amount);
  const receivedShare = total > 0 ? Math.min(100, Math.max(0, (received / total) * 100)) : 0;

  return (
    <div className="debt-row-progress">
      <div className="debt-row-meter" aria-hidden="true">
        <span
          className="debt-row-meter-segment debt-row-meter-paid"
          style={{ width: `${receivedShare}%` }}
        />
        <span
          className="debt-row-meter-segment debt-row-meter-remaining"
          style={{ width: `${100 - receivedShare}%` }}
        />
      </div>
      <span className="debt-row-caption">{formatBRL(receivable.remaining_amount)} a receber</span>
    </div>
  );
}

export function ReceivableList({
  receivables,
  isLoading,
  onOpen,
  onEdit,
  onDelete,
}: {
  receivables: Receivable[];
  isLoading: boolean;
  onOpen: (receivable: Receivable) => void;
  onEdit: (receivable: Receivable) => void;
  onDelete: (receivable: Receivable) => void;
}) {
  const columns: ProColumns<Receivable>[] = [
    { title: "Nome", dataIndex: "name" },
    {
      title: "Progresso",
      dataIndex: "remaining_amount",
      render: (_, receivable) => <ReceivableProgress receivable={receivable} />,
    },
    {
      title: "Situação",
      dataIndex: "status",
      render: (_, receivable) => (
        <Tag color={receivableStatusColor[receivable.status]}>{receivableStatusLabel[receivable.status]}</Tag>
      ),
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, receivable) => (
        <Space onClick={(event) => event.stopPropagation()}>
          <Button type="link" onClick={() => onEdit(receivable)}>
            Editar
          </Button>
          <Popconfirm
            title="Excluir conta a receber"
            description={deleteDescription(receivable)}
            onConfirm={() => onDelete(receivable)}
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
    <ProTable<Receivable>
      aria-label="Contas a receber"
      columns={columns}
      dataSource={receivables}
      loading={isLoading}
      rowKey="id"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
      locale={{
        emptyText: (
          <div className="debt-list-empty">
            <DollarOutlined className="debt-list-empty-icon" aria-hidden="true" />
            <span>Nenhuma conta a receber cadastrada ainda.</span>
          </div>
        ),
      }}
      onRow={(receivable) => ({
        onClick: () => onOpen(receivable),
        style: { cursor: "pointer" },
      })}
    />
  );
}
