import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { Alert, Button, Checkbox, Drawer, Flex, Input, Select, Switch } from "antd";
import { useEffect, useState } from "react";

import type {
  AutomationConditionField,
  AutomationConditionOperator,
  AutomationLogicOperator,
  AutomationRule,
  AutomationRuleCondition,
  AutomationRuleConditionOptions,
  AutomationRuleWrite,
} from "../../api/contracts";
import {
  conditionFieldLabel,
  conditionOperatorLabel,
  logicOperatorLabel,
} from "../../presentation/automationRuleLabels";

const fieldOptions = (Object.keys(conditionFieldLabel) as AutomationConditionField[]).map(
  (value) => ({ value, label: conditionFieldLabel[value] }),
);
const operatorOptions = (
  Object.keys(conditionOperatorLabel) as AutomationConditionOperator[]
).map((value) => ({ value, label: conditionOperatorLabel[value] }));
const logicOptions = (Object.keys(logicOperatorLabel) as AutomationLogicOperator[]).map(
  (value) => ({ value, label: logicOperatorLabel[value] }),
);

const blankCondition: AutomationRuleCondition = {
  field: "description",
  operator: "contains",
  value: "",
};

function draftFrom(rule: AutomationRule | null): {
  name: string;
  isActive: boolean;
  logicOperator: AutomationLogicOperator;
  conditions: AutomationRuleCondition[];
} {
  return rule
    ? {
        name: rule.name,
        isActive: rule.is_active,
        logicOperator: rule.logic_operator,
        conditions: rule.conditions.map((condition) => ({ ...condition })),
      }
    : { name: "", isActive: true, logicOperator: "or", conditions: [{ ...blankCondition }] };
}

export function AutomationRuleForm({
  open,
  rule,
  conditionOptions,
  conditionOptionsLoading,
  submitting,
  submitError,
  onSubmit,
  onCancel,
}: {
  open: boolean;
  rule: AutomationRule | null;
  conditionOptions: AutomationRuleConditionOptions;
  conditionOptionsLoading: boolean;
  submitting: boolean;
  submitError: string | null;
  onSubmit: (write: AutomationRuleWrite) => void;
  onCancel: () => void;
}) {
  const [draft, setDraft] = useState(() => draftFrom(rule));
  const [applyRetroactively, setApplyRetroactively] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setDraft(draftFrom(rule));
      setApplyRetroactively(false);
      setError(null);
    }
  }, [open, rule]);

  const updateCondition = (index: number, patch: Partial<AutomationRuleCondition>) => {
    setDraft((current) => ({
      ...current,
      conditions: current.conditions.map((condition, current_index) =>
        current_index === index ? { ...condition, ...patch } : condition,
      ),
    }));
  };

  const removeCondition = (index: number) => {
    setDraft((current) => ({
      ...current,
      conditions: current.conditions.filter((_, current_index) => current_index !== index),
    }));
  };

  const addCondition = () => {
    setDraft((current) => ({
      ...current,
      conditions: [...current.conditions, { ...blankCondition }],
    }));
  };

  const valueOptions = (field: AutomationConditionField) =>
    (field === "account" ? conditionOptions.accounts : conditionOptions.cards).map(
      (value) => ({ value, label: value }),
    );

  const submit = () => {
    if (draft.name.trim() === "") {
      setError("Informe um nome para a regra.");
      return;
    }
    if (draft.conditions.length === 0) {
      setError("Adicione ao menos uma condição.");
      return;
    }
    if (draft.conditions.some((condition) => condition.value.trim() === "")) {
      setError("Preencha o valor de todas as condições.");
      return;
    }
    setError(null);
    onSubmit({
      name: draft.name.trim(),
      is_active: draft.isActive,
      logic_operator: draft.logicOperator,
      conditions: draft.conditions.map((condition) => ({
        ...condition,
        value: condition.value.trim(),
      })),
      apply_retroactively: applyRetroactively,
    });
  };

  return (
    <Drawer
      title={rule ? "Editar regra" : "Nova regra"}
      open={open}
      onClose={onCancel}
      width={480}
      destroyOnHidden
      footer={
        <Flex justify="end" gap="small">
          <Button onClick={onCancel}>Cancelar</Button>
          <Button type="primary" loading={submitting} onClick={submit}>
            Salvar
          </Button>
        </Flex>
      }
    >
      <Flex vertical gap="middle" className="automation-rule-form">
        {(error ?? submitError) && <Alert type="error" showIcon message={error ?? submitError} />}

        <div className="filter-field">
          <label htmlFor="automation-rule-name">Nome</label>
          <Input
            id="automation-rule-name"
            value={draft.name}
            onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
            placeholder="Ex.: Ignorar assinaturas"
          />
        </div>

        <Flex align="center" gap="small">
          <Switch
            id="automation-rule-active"
            checked={draft.isActive}
            onChange={(checked) => setDraft((current) => ({ ...current, isActive: checked }))}
          />
          <label htmlFor="automation-rule-active">Ativa</label>
        </Flex>

        <div className="filter-field">
          <label htmlFor="automation-rule-logic">Combinar condições com</label>
          <Select
            id="automation-rule-logic"
            value={draft.logicOperator}
            options={logicOptions}
            onChange={(value: AutomationLogicOperator) =>
              setDraft((current) => ({ ...current, logicOperator: value }))
            }
          />
        </div>

        <Flex vertical gap="small">
          <span>Condições</span>
          {draft.conditions.map((condition, index) => (
            <Flex key={index} gap="small" align="center" className="automation-condition-row">
              <Select
                aria-label="Campo"
                value={condition.field}
                options={fieldOptions}
                onChange={(value: AutomationConditionField) =>
                  updateCondition(index, { field: value, value: "" })
                }
              />
              <Select
                aria-label="Operador"
                value={condition.operator}
                options={operatorOptions}
                onChange={(value: AutomationConditionOperator) =>
                  updateCondition(index, { operator: value })
                }
              />
              {condition.field === "description" ? (
                <Input
                  aria-label="Valor"
                  value={condition.value}
                  onChange={(event) => updateCondition(index, { value: event.target.value })}
                  placeholder="Valor"
                />
              ) : (
                <Select
                  aria-label="Valor"
                  value={condition.value || undefined}
                  options={valueOptions(condition.field)}
                  loading={conditionOptionsLoading}
                  showSearch
                  optionFilterProp="label"
                  placeholder={
                    condition.field === "account"
                      ? "Selecione uma conta"
                      : "Selecione um cartão"
                  }
                  notFoundContent={
                    conditionOptionsLoading
                      ? "Carregando…"
                      : condition.field === "account"
                        ? "Nenhuma conta encontrada"
                        : "Nenhum cartão encontrado"
                  }
                  onChange={(value: string) => updateCondition(index, { value })}
                />
              )}
              <Button
                aria-label="Remover condição"
                icon={<DeleteOutlined aria-hidden="true" />}
                disabled={draft.conditions.length <= 1}
                onClick={() => removeCondition(index)}
              />
            </Flex>
          ))}
          <Button icon={<PlusOutlined aria-hidden="true" />} onClick={addCondition}>
            Adicionar condição
          </Button>
        </Flex>

        <Checkbox
          checked={applyRetroactively}
          onChange={(event) => setApplyRetroactively(event.target.checked)}
        >
          Aplicar agora às transações existentes que casarem com as condições
        </Checkbox>
      </Flex>
    </Drawer>
  );
}
