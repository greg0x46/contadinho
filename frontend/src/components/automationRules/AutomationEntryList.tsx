import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Popconfirm, Space, Switch, Tag } from "antd";

import type { AutomationActionType, AutomationRule, Category, RecurringCommitment } from "../../api/contracts";
import { summarizeConditions } from "../../presentation/ruleConditionLabels";

type EntryRow = {
  key: string;
  rule: AutomationRule;
};

const actionLabel: Record<AutomationActionType, string> = {
  ignore: "Ignorar",
  reconcile: "Conciliar",
  set_category: "Aplicar categoria",
};

const actionColor: Record<AutomationActionType, string> = {
  ignore: "default",
  reconcile: "blue",
  set_category: "green",
};

export function AutomationEntryList({
  rules,
  commitments,
  categories,
  isLoading,
  togglingRuleId,
  onEditRule,
  onToggleRule,
  onDeleteRule,
}: {
  rules: AutomationRule[];
  commitments: RecurringCommitment[];
  categories: Category[];
  isLoading: boolean;
  togglingRuleId: string | null;
  onEditRule: (rule: AutomationRule) => void;
  onToggleRule: (rule: AutomationRule, isActive: boolean) => void;
  onDeleteRule: (rule: AutomationRule) => void;
}) {
  const commitmentName = (scenarioId: string) =>
    commitments.find((commitment) => commitment.id === scenarioId)?.name ??
    "Recorrência removida";
  const categoryName = (categoryId: string) =>
    categories.find((category) => category.id === categoryId)?.name ?? "Categoria removida";

  const rows: EntryRow[] = rules.map((rule) => ({ key: rule.id, rule }));

  const columns: ProColumns<EntryRow>[] = [
    { title: "Nome", dataIndex: ["rule", "name"] },
    {
      title: "Ações",
      key: "actions",
      render: (_, row) => (
        <Space size={4} wrap>
          {row.rule.actions.map((action) => (
            <Tag key={action.type} color={actionColor[action.type]}>
              {action.type === "reconcile" && action.scenario_id
                ? `Concilia: ${commitmentName(action.scenario_id)}`
                : action.type === "set_category" && action.category_id
                  ? `Categoriza: ${categoryName(action.category_id)}`
                  : actionLabel[action.type]}
            </Tag>
          ))}
        </Space>
      ),
    },
    {
      title: "Condições",
      key: "summary",
      render: (_, row) => summarizeConditions(row.rule.conditions, row.rule.logic_operator),
    },
    {
      title: "Ativa",
      dataIndex: ["rule", "is_active"],
      render: (_, row) => (
        <Switch
          aria-label={`${row.rule.is_active ? "Desativar" : "Ativar"} automação ${row.rule.name}`}
          checked={row.rule.is_active}
          loading={togglingRuleId === row.rule.id}
          onChange={(checked) => onToggleRule(row.rule, checked)}
        />
      ),
    },
    {
      title: "Opções",
      key: "options",
      valueType: "option",
      render: (_, row) => (
        <Space>
          <Button type="link" onClick={() => onEditRule(row.rule)}>
            Editar
          </Button>
          <Popconfirm
            title="Excluir automação"
            description={
              row.rule.actions.some((action) => action.type === "reconcile")
                ? "A recorrência vinculada não será excluída, apenas deixará de ser conciliada automaticamente. Transações já ignoradas/categorizadas por esta regra permanecem como estão."
                : "Transações já ignoradas/categorizadas por esta regra permanecem como estão."
            }
            onConfirm={() => onDeleteRule(row.rule)}
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
    <ProTable<EntryRow>
      aria-label="Automações"
      columns={columns}
      dataSource={rows}
      loading={isLoading}
      rowKey="key"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
      locale={{ emptyText: "Nenhuma automação criada ainda." }}
    />
  );
}
