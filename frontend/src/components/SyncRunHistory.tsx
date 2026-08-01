import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Space } from "antd";
import { Link } from "react-router-dom";

import type { SyncRun } from "../api/contracts";
import { formatDate } from "../presentation/dates";
import { SyncStatusBadge } from "./SyncStatusBadge";

export function SyncRunHistory({ runs }: { runs: SyncRun[] }) {
  const columns: ProColumns<SyncRun>[] = [
    {
      title: "Início",
      dataIndex: "started_at",
      render: (_, run) => <time dateTime={run.started_at}>{formatDate(run.started_at)}</time>,
    },
    {
      title: "Situação",
      dataIndex: "status",
      render: (_, run) => <SyncStatusBadge status={run.status} />,
    },
    { title: "Contas processadas", dataIndex: "accounts_processed", align: "right" },
    { title: "Transações incluídas", dataIndex: "transactions_inserted", align: "right" },
    { title: "Transações atualizadas", dataIndex: "transactions_updated", align: "right" },
    {
      title: "Ação",
      valueType: "option",
      render: (_, run) => (
        <Space>
          <Link
            aria-label={`Ver detalhes da sincronização iniciada em ${formatDate(run.started_at)}`}
            to={`/open-banking/sync-runs/${run.id}`}
          >
            Ver detalhes
          </Link>
        </Space>
      ),
    },
  ];

  return (
    <ProTable<SyncRun>
      aria-label="Sincronizações recentes"
      columns={columns}
      dataSource={runs}
      rowKey="id"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
    />
  );
}
