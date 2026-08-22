import { PlusOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button } from "antd";
import { useState } from "react";

import type { AutomationAction, AutomationRule, RetroactiveApplyResult } from "../api/contracts";
import {
  AutomationEntry,
  AutomationEntryForm,
  AutomationEntrySubmitPayload,
} from "../components/automationRules/AutomationEntryForm";
import { AutomationEntryList } from "../components/automationRules/AutomationEntryList";
import { useAutomationRules } from "../hooks/useAutomationRules";
import { useCategories } from "../hooks/useCategories";
import { useRecurringCommitments } from "../hooks/useRecurringCommitments";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar a automação.";
}

function retroactiveMessage(result: RetroactiveApplyResult): string {
  const changes: string[] = [];
  if (result.ignored > 0) changes.push(`${result.ignored} ignorada(s)`);
  if (result.categorized > 0) changes.push(`${result.categorized} categorizada(s)`);
  return changes.length > 0
    ? `${result.matched} transação(ões) encontrada(s), ${changes.join(", ")} agora.`
    : `${result.matched} transação(ões) encontrada(s), nenhuma alterada (já processadas ou com decisão manual).`;
}

export function AutomationRulesPage() {
  const automationRules = useAutomationRules();
  const commitments = useRecurringCommitments();
  const categories = useCategories();
  const [formOpen, setFormOpen] = useState(false);
  const [editingEntry, setEditingEntry] = useState<AutomationEntry | null>(null);
  const [togglingRuleId, setTogglingRuleId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [lastResult, setLastResult] = useState<RetroactiveApplyResult | null>(null);

  const openCreate = () => {
    setEditingEntry(null);
    setSaveError(null);
    setFormOpen(true);
  };

  const openEditRule = (rule: AutomationRule) => {
    const target = rule.actions.find((action) => action.type === "reconcile");
    const linkedCommitment = target
      ? commitments.commitments.find(
          (commitment) =>
            (target.recurring_commitment_id !== null && commitment.id === target.recurring_commitment_id) ||
            (target.scenario_id !== undefined &&
              target.scenario_id !== null &&
              commitment.scenario_id === target.scenario_id),
        ) ?? null
      : null;
    setEditingEntry({ rule, linkedCommitment });
    setSaveError(null);
    setFormOpen(true);
  };

  const closeForm = () => setFormOpen(false);

  const submit = async (payload: AutomationEntrySubmitPayload) => {
    setSaveError(null);
    setSaving(true);
    try {
      const editingRuleId = editingEntry?.rule.id ?? null;

      const actions: AutomationAction[] = [];
      if (payload.actionTypes.includes("ignore")) {
        actions.push({ type: "ignore", recurring_commitment_id: null, category_id: null });
      }
      if (payload.actionTypes.includes("set_category") && payload.categoryId) {
        actions.push({ type: "set_category", recurring_commitment_id: null, category_id: payload.categoryId });
      }
      if (payload.actionTypes.includes("reconcile") && payload.reconcile) {
        const { reconcile } = payload;
        const savedCommitment =
          reconcile.commitmentMode === "existing" && reconcile.existingCommitmentId
            ? (
                await commitments.updateCommitment({
                  commitmentId: reconcile.existingCommitmentId,
                  write: reconcile.commitment,
                })
              )
            : await commitments.createCommitment(reconcile.commitment);
        const reconcileAction: AutomationAction = {
          type: "reconcile",
          recurring_commitment_id: savedCommitment.id,
          category_id: null,
        };
        if (savedCommitment.scenario_id !== undefined) {
          reconcileAction.scenario_id = savedCommitment.scenario_id;
        }
        actions.push(reconcileAction);
      }

      const write = {
        name: payload.name,
        is_active: payload.isActive,
        logic_operator: payload.logicOperator,
        conditions: payload.conditions,
        actions,
        apply_retroactively: payload.applyRetroactively,
      };
      const result = editingRuleId
        ? await automationRules.updateRule({ ruleId: editingRuleId, write })
        : await automationRules.createRule(write);
      setLastResult(result.retroactive_apply);
      setFormOpen(false);
    } catch (error) {
      setSaveError(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const toggleRule = async (rule: AutomationRule, isActive: boolean) => {
    setActionError(null);
    setTogglingRuleId(rule.id);
    try {
      await automationRules.toggleRule({ ruleId: rule.id, isActive });
    } catch (error) {
      setActionError(errorMessage(error));
    } finally {
      setTogglingRuleId(null);
    }
  };

  const removeRule = async (rule: AutomationRule) => {
    setActionError(null);
    try {
      await automationRules.deleteRule(rule.id);
    } catch (error) {
      setActionError(errorMessage(error));
    }
  };

  const loadError = automationRules.error ?? commitments.error;

  return (
    <PageContainer
      title="Automações"
      subTitle="Regras que ignoram transações e automações de conciliação de recorrências, automaticamente"
      content="Crie automações no estilo de filtros de e-mail: ignore transações recorrentes sem revisar cada sincronização, ou concilie compromissos recorrentes (salário, aluguel, assinaturas) com as transações reais."
      extra={[
        <Button key="new-entry" type="primary" icon={<PlusOutlined aria-hidden="true" />} onClick={openCreate}>
          Nova automação
        </Button>,
      ]}
    >
      {lastResult && (
        <Alert
          type="success"
          showIcon
          closable
          onClose={() => setLastResult(null)}
          message={retroactiveMessage(lastResult)}
          style={{ marginBottom: 16 }}
        />
      )}
      {actionError && (
        <Alert
          type="error"
          showIcon
          closable
          onClose={() => setActionError(null)}
          message={actionError}
          style={{ marginBottom: 16 }}
        />
      )}
      {loadError && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as automações"
          description={
            <Button
              onClick={() => {
                automationRules.refetch();
                commitments.refetch();
              }}
            >
              Tentar novamente
            </Button>
          }
          style={{ marginBottom: 16 }}
        />
      )}
      <AutomationEntryList
        rules={automationRules.rules}
        commitments={commitments.commitments}
        categories={categories.categories}
        isLoading={automationRules.isLoading || commitments.isLoading}
        togglingRuleId={togglingRuleId}
        onEditRule={openEditRule}
        onToggleRule={toggleRule}
        onDeleteRule={removeRule}
      />
      <AutomationEntryForm
        open={formOpen}
        entry={editingEntry}
        conditionOptions={automationRules.conditionOptions}
        conditionOptionsLoading={automationRules.areConditionOptionsLoading}
        categories={categories.categories}
        commitments={commitments.commitments}
        commitmentsLoading={commitments.isLoading}
        submitting={saving}
        submitError={saveError}
        onSubmit={submit}
        onCancel={closeForm}
      />
    </PageContainer>
  );
}
