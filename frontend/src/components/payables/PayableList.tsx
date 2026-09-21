import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Popconfirm, Skeleton, Space, Tag } from "antd";

import type { Payable } from "../../api/contracts";
import { payableStatusColor, payableStatusLabel, payableVocabulary } from "../../presentation/payableLabels";
import { formatBRL } from "../../presentation/money";
import { useCompactScreen } from "../shared/useCompactScreen";

function deleteDescription(payable: Payable): string {
  if (payable.link_count === 0) {
    return "Esta ação não pode ser desfeita.";
  }
  const plural = payable.link_count > 1;
  return `${payable.link_count} transação${plural ? "ões" : ""} vinculada${plural ? "s" : ""} ${
    plural ? "serão desfeitas" : "será desfeita"
  }; as transações em si permanecem inalteradas.`;
}

function PayableProgress({ payable }: { payable: Payable }) {
  const vocab = payableVocabulary[payable.kind];
  const total = Number(payable.total_amount);
  const settled = Number(payable.settled_amount);
  const settledShare = total > 0 ? Math.min(100, Math.max(0, (settled / total) * 100)) : 0;

  return (
    <div className="debt-row-progress">
      <div className="debt-row-meter" aria-hidden="true">
        <span
          className="debt-row-meter-segment debt-row-meter-paid"
          style={{ width: `${settledShare}%` }}
        />
        <span
          className="debt-row-meter-segment debt-row-meter-remaining"
          style={{ width: `${100 - settledShare}%` }}
        />
      </div>
      <span className="debt-row-caption">
        {formatBRL(payable.remaining_amount)} {vocab.remainingSuffix}
      </span>
    </div>
  );
}

type Props = {
  payables: Payable[];
  isLoading: boolean;
  onOpen: (payable: Payable) => void;
  onEdit: (payable: Payable) => void;
  onDelete: (payable: Payable) => void;
};

function PayableActions({ payable, onEdit, onDelete }: Pick<Props, "onEdit" | "onDelete"> & { payable: Payable }) {
  return (
    <Space onClick={(event) => event.stopPropagation()}>
      <Button type="link" onClick={() => onEdit(payable)}>
        Editar
      </Button>
      <Popconfirm
        title={payableVocabulary[payable.kind].deleteTitle}
        description={deleteDescription(payable)}
        onConfirm={() => onDelete(payable)}
        okText="Excluir"
        cancelText="Cancelar"
      >
        <Button type="link" danger>
          Excluir
        </Button>
      </Popconfirm>
    </Space>
  );
}

const emptyText = <span>Nenhuma dívida ou conta a receber cadastrada ainda.</span>;

/**
 * On a phone the table would scroll sideways, so each payable becomes a
 * stacked row: name and kind, the progress meter, the status and the same
 * actions the table offers.
 */
function CompactPayableList({ payables, isLoading, onOpen, onEdit, onDelete }: Props) {
  if (isLoading) {
    return (
      <div className="debt-list-compact" role="status" aria-label="Carregando pendências">
        <Skeleton active paragraph={{ rows: 3 }} />
      </div>
    );
  }
  if (payables.length === 0) {
    return <div className="debt-list-empty">{emptyText}</div>;
  }
  return (
    <ul className="debt-list-compact" aria-label="Pendências">
      {payables.map((payable) => (
        <li key={payable.id} className="debt-item">
          <button type="button" className="debt-item-main" onClick={() => onOpen(payable)}>
            <span className="debt-item-heading">
              <span className="debt-item-name">{payable.name}</span>
              <Tag color={payableStatusColor[payable.status]}>{payableStatusLabel[payable.kind][payable.status]}</Tag>
            </span>
            <span className="debt-item-kind">
              {payableVocabulary[payable.kind].icon} {payable.kind === "debt" ? "Dívida" : "A receber"}
            </span>
            <PayableProgress payable={payable} />
          </button>
          <div className="debt-item-actions">
            <PayableActions payable={payable} onEdit={onEdit} onDelete={onDelete} />
          </div>
        </li>
      ))}
    </ul>
  );
}

export function PayableList(props: Props) {
  const compact = useCompactScreen();
  return compact ? <CompactPayableList {...props} /> : <PayableTable {...props} />;
}

function PayableTable({ payables, isLoading, onOpen, onEdit, onDelete }: Props) {
  const columns: ProColumns<Payable>[] = [
    {
      title: "Tipo",
      dataIndex: "kind",
      filters: [
        { text: "Dívida", value: "debt" },
        { text: "A receber", value: "receivable" },
      ],
      onFilter: (value, payable) => payable.kind === value,
      render: (_, payable) => (
        <Space size="small">
          {payableVocabulary[payable.kind].icon}
          {payable.kind === "debt" ? "Dívida" : "A receber"}
        </Space>
      ),
    },
    { title: "Nome", dataIndex: "name" },
    {
      title: "Progresso",
      dataIndex: "remaining_amount",
      render: (_, payable) => <PayableProgress payable={payable} />,
    },
    {
      title: "Situação",
      dataIndex: "status",
      render: (_, payable) => (
        <Tag color={payableStatusColor[payable.status]}>{payableStatusLabel[payable.kind][payable.status]}</Tag>
      ),
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, payable) => <PayableActions payable={payable} onEdit={onEdit} onDelete={onDelete} />,
    },
  ];

  return (
    <ProTable<Payable>
      aria-label="Pendências"
      columns={columns}
      dataSource={payables}
      loading={isLoading}
      rowKey="id"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
      locale={{ emptyText: <div className="debt-list-empty">{emptyText}</div> }}
      onRow={(payable) => ({
        onClick: () => onOpen(payable),
        style: { cursor: "pointer" },
      })}
    />
  );
}
