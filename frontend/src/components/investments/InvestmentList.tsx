import { FundOutlined } from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Tag, Tooltip, Typography } from "antd";

import type { Investment } from "../../api/contracts";
import { investmentTypeLabel, investmentYield } from "../../presentation/investmentLabels";
import { formatBRL } from "../../presentation/money";

function YieldCell({ investment }: { investment: Investment }) {
  const estimate = investmentYield(investment);
  if (estimate === null) {
    return <Typography.Text type="secondary">Não disponível</Typography.Text>;
  }
  const negative = estimate.value.startsWith("-");
  return (
    <Tooltip
      title={
        estimate.source === "informado"
          ? "Informado pela instituição"
          : "Calculado a partir do histórico de aplicações e resgates"
      }
    >
      <span style={{ color: negative ? "#f5222d" : "#1baf7a", fontVariantNumeric: "tabular-nums" }}>
        {formatBRL(estimate.value)}
      </span>
    </Tooltip>
  );
}

export function InvestmentList({
  investments,
  isLoading,
  onOpen,
}: {
  investments: Investment[];
  isLoading: boolean;
  onOpen: (investment: Investment) => void;
}) {
  const columns: ProColumns<Investment>[] = [
    { title: "Nome", dataIndex: "name", render: (_, i) => i.name ?? "Sem nome" },
    { title: "Instituição", dataIndex: "source_display_name", render: (_, i) => i.source_display_name ?? "—" },
    {
      title: "Tipo",
      dataIndex: "investment_type",
      render: (_, i) => <Tag>{investmentTypeLabel(i.investment_type)}</Tag>,
    },
    {
      title: "Saldo atual",
      dataIndex: "balance",
      render: (_, i) => (i.balance !== null ? formatBRL(i.balance) : "—"),
    },
    {
      title: "Rendimento",
      dataIndex: "amount_profit",
      render: (_, i) => <YieldCell investment={i} />,
    },
  ];

  return (
    <ProTable<Investment>
      aria-label="Investimentos"
      columns={columns}
      dataSource={investments}
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
            <FundOutlined className="debt-list-empty-icon" aria-hidden="true" />
            <span>Nenhum investimento sincronizado ainda.</span>
          </div>
        ),
      }}
      onRow={(investment) => ({
        onClick: () => onOpen(investment),
        style: { cursor: "pointer" },
      })}
    />
  );
}
