import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Popconfirm, Space, Switch, Tag } from "antd";

import type { AutomationActionType, AutomationRule, RecurringCommitment } from "../../api/contracts";
import { summarizeConditions } from "../../presentation/ruleConditionLabels";

type EntryRow = {
  key: string;
  rule: AutomationRule;
  action: AutomationActionType;
  linkedCommitmentName: string | null;
};

const actionLabel: Record<AutomationActionType, string> = {
  ignore: "Ignorar",
  reconcile: "Conciliar",
};

const actionColor: Record<AutomationActionType, string> = {
  ignore: "default",
  reconcile: "blue",
};

export function AutomationEntryList({
  rules,
  commitments,
  isLoading,
  togglingRuleId,
  onEditRule,
  onToggleRule,
  onDeleteRule,
}: {
  rules: AutomationRule[];
  commitments: RecurringCommitment[];
  isLoading: boolean;
  togglingRuleId: string | null;
  onEditRule: (rule: AutomationRule) => void;
  onToggleRule: (rule: AutomationRule, isActive: boolean) => void;
  onDeleteRule: (rule: AutomationRule) => void;
}) {
  const commitmentName = (commitmentId: string) =>
    commitments.find((commitment) => commitment.id === commitmentId)?.name ?? "Recorrência removida";

  const rows: EntryRow[] = rules.map((rule) => {
    const action = rule.actions[0]?.type ?? "ignore";
    const targetId = rule.actions[0]?.recurring_commitment_id ?? null;
    return {
      key: rule.id,
      rule,
      action,
      linkedCommitmentName: targetId ? commitmentName(targetId) : null,
    };
  });

  const columns: ProColumns<EntryRow>[] = [
    { title: "Nome", dataIndex: ["rule", "name"] },
    {
      title: "Ação",
      dataIndex: "action",
      render: (_, row) => (
        <Tag color={actionColor[row.action]}>
          {row.action === "reconcile" && row.linkedCommitmentName
            ? `Concilia: ${row.linkedCommitmentName}`
            : actionLabel[row.action]}
        </Tag>
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
              row.action === "ignore"
                ? "Transações já ignoradas por esta regra permanecem ignoradas."
                : "A recorrência vinculada não será excluída, apenas deixará de ser conciliada automaticamente."
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
