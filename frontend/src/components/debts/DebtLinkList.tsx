import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Popconfirm } from "antd";

import type { DebtLinkedTransaction } from "../../api/contracts";
import { formatOptionalDate } from "../../presentation/dates";
import { formatBRL } from "../../presentation/money";

export function DebtLinkList({
  links,
  unlinkingLinkId,
  onUnlink,
}: {
  links: DebtLinkedTransaction[];
  unlinkingLinkId: string | null;
  onUnlink: (link: DebtLinkedTransaction) => void;
}) {
  const columns: ProColumns<DebtLinkedTransaction>[] = [
    {
      title: "Data",
      dataIndex: "occurred_at",
      render: (_, link) => formatOptionalDate(link.occurred_at),
    },
    { title: "Descrição", dataIndex: "description", render: (_, link) => link.description ?? "—" },
    {
      title: "Valor",
      dataIndex: "current_amount",
      render: (_, link) => formatBRL(link.current_amount),
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, link) => (
        <Popconfirm
          title="Desvincular transação"
          description="A transação permanece inalterada e volta a ficar disponível para vincular."
          onConfirm={() => onUnlink(link)}
          okText="Desvincular"
          cancelText="Cancelar"
        >
          <Button type="link" danger loading={unlinkingLinkId === link.id}>
            Desvincular
          </Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <ProTable<DebtLinkedTransaction>
      aria-label="Transações vinculadas"
      columns={columns}
      dataSource={links}
      rowKey="id"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
      locale={{ emptyText: "Nenhuma transação vinculada ainda." }}
    />
  );
}
