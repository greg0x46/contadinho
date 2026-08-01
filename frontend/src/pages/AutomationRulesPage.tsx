import { PlusOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button } from "antd";
import { useState } from "react";

import type { AutomationRule, AutomationRuleWrite, RetroactiveApplyResult } from "../api/contracts";
import { AutomationRuleForm } from "../components/automationRules/AutomationRuleForm";
import { AutomationRuleList } from "../components/automationRules/AutomationRuleList";
import { useAutomationRules } from "../hooks/useAutomationRules";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar a regra.";
}

function retroactiveMessage(result: RetroactiveApplyResult): string {
  return result.ignored > 0
    ? `${result.matched} transação(ões) encontrada(s), ${result.ignored} ignorada(s) agora.`
    : `${result.matched} transação(ões) encontrada(s), nenhuma alterada (já ignoradas ou com decisão manual).`;
}

export function AutomationRulesPage() {
  const automationRules = useAutomationRules();
  const [formOpen, setFormOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<AutomationRule | null>(null);
  const [togglingRuleId, setTogglingRuleId] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [lastResult, setLastResult] = useState<RetroactiveApplyResult | null>(null);

  const openCreate = () => {
    setEditingRule(null);
    setSaveError(null);
    setFormOpen(true);
  };

  const openEdit = (rule: AutomationRule) => {
    setEditingRule(rule);
    setSaveError(null);
    setFormOpen(true);
  };

  const closeForm = () => setFormOpen(false);

  const submit = async (write: AutomationRuleWrite) => {
    setSaveError(null);
    try {
      const result = editingRule
        ? await automationRules.updateRule({ ruleId: editingRule.id, write })
        : await automationRules.createRule(write);
      setLastResult(result.retroactive_apply);
      setFormOpen(false);
    } catch (error) {
      setSaveError(errorMessage(error));
    }
  };

  const toggle = async (rule: AutomationRule, isActive: boolean) => {
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

  const remove = async (rule: AutomationRule) => {
    setActionError(null);
    try {
      await automationRules.deleteRule(rule.id);
    } catch (error) {
      setActionError(errorMessage(error));
    }
  };

  return (
    <PageContainer
      title="Automações"
      subTitle="Regras que ignoram transações automaticamente"
      content="Crie regras no estilo de filtros de e-mail para ignorar transações recorrentes sem revisar cada sincronização."
      extra={[
        <Button key="new-rule" type="primary" icon={<PlusOutlined aria-hidden="true" />} onClick={openCreate}>
          Nova regra
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
      {automationRules.error && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as regras de automação"
          description={<Button onClick={() => automationRules.refetch()}>Tentar novamente</Button>}
          style={{ marginBottom: 16 }}
        />
      )}
      <AutomationRuleList
        rules={automationRules.rules}
        isLoading={automationRules.isLoading}
        togglingRuleId={togglingRuleId}
        onEdit={openEdit}
        onToggle={toggle}
        onDelete={remove}
      />
      <AutomationRuleForm
        open={formOpen}
        rule={editingRule}
        conditionOptions={automationRules.conditionOptions}
        conditionOptionsLoading={automationRules.areConditionOptionsLoading}
        submitting={automationRules.isSaving}
        submitError={saveError}
        onSubmit={submit}
        onCancel={closeForm}
      />
    </PageContainer>
  );
}
