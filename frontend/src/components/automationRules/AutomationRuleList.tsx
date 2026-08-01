import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Popconfirm, Space, Switch } from "antd";

import type { AutomationRule } from "../../api/contracts";
import { logicOperatorLabel, summarizeConditions } from "../../presentation/automationRuleLabels";

export function AutomationRuleList({
  rules,
  isLoading,
  togglingRuleId,
  onEdit,
  onToggle,
  onDelete,
}: {
  rules: AutomationRule[];
  isLoading: boolean;
  togglingRuleId: string | null;
  onEdit: (rule: AutomationRule) => void;
  onToggle: (rule: AutomationRule, isActive: boolean) => void;
  onDelete: (rule: AutomationRule) => void;
}) {
  const columns: ProColumns<AutomationRule>[] = [
    { title: "Nome", dataIndex: "name" },
    {
      title: "Condições",
      dataIndex: "conditions",
      render: (_, rule) => summarizeConditions(rule.conditions, rule.logic_operator),
    },
    {
      title: "Operador",
      dataIndex: "logic_operator",
      render: (_, rule) => logicOperatorLabel[rule.logic_operator],
    },
    {
      title: "Ativa",
      dataIndex: "is_active",
      render: (_, rule) => (
        <Switch
          aria-label={`${rule.is_active ? "Desativar" : "Ativar"} regra ${rule.name}`}
          checked={rule.is_active}
          loading={togglingRuleId === rule.id}
          onChange={(checked) => onToggle(rule, checked)}
        />
      ),
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, rule) => (
        <Space>
          <Button type="link" onClick={() => onEdit(rule)}>
            Editar
          </Button>
          <Popconfirm
            title="Excluir regra"
            description="Transações já ignoradas por esta regra permanecem ignoradas."
            onConfirm={() => onDelete(rule)}
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
    <ProTable<AutomationRule>
      aria-label="Regras de automação"
      columns={columns}
      dataSource={rules}
      loading={isLoading}
      rowKey="id"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
      locale={{ emptyText: "Nenhuma regra criada ainda." }}
    />
  );
}
