import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { Button, Flex, Input, Select } from "antd";

import type { RuleCondition, RuleConditionField, RuleConditionOperator } from "../../api/contracts";

/**
 * Per-field rendering rules for one row of the shared condition-list
 * builder (used by AutomationRuleForm and RecurringCommitmentForm — see
 * .specs/relatorio-financeiro/m0-motor-de-regras.md). A field is either:
 * - a literal text-match field (operator select + free/select value), or
 * - a "tolerance" field (amount/day_of_month): the operator is implied by
 *   the field itself, and the only input is a numeric tolerance, since the
 *   reference (expected amount/day) is always the occurrence being
 *   resolved — never something the user types in.
 */
export interface ConditionFieldConfig {
  field: RuleConditionField;
  label: string;
  operators: { value: RuleConditionOperator; label: string }[];
  toleranceOperator?: RuleConditionOperator;
  toleranceSuffix?: string;
  valueOptions?: (field: RuleConditionField) => { value: string; label: string }[];
  valueOptionsLoading?: boolean;
  valuePlaceholder?: (field: RuleConditionField) => string;
  valueNotFoundContent?: (field: RuleConditionField) => string;
}

function blankConditionFor(config: ConditionFieldConfig): RuleCondition {
  return {
    field: config.field,
    operator: config.toleranceOperator ?? config.operators[0].value,
    value: "",
  };
}

export function ConditionListEditor({
  conditions,
  onChange,
  fieldConfigs,
  minConditions = 1,
}: {
  conditions: RuleCondition[];
  onChange: (conditions: RuleCondition[]) => void;
  fieldConfigs: ConditionFieldConfig[];
  minConditions?: number;
}) {
  const fieldOptions = fieldConfigs.map((config) => ({ value: config.field, label: config.label }));
  const configFor = (field: RuleConditionField) =>
    fieldConfigs.find((config) => config.field === field) ?? fieldConfigs[0];

  const updateCondition = (index: number, patch: Partial<RuleCondition>) => {
    onChange(conditions.map((condition, i) => (i === index ? { ...condition, ...patch } : condition)));
  };

  const changeField = (index: number, field: RuleConditionField) => {
    const config = configFor(field);
    updateCondition(index, { field, operator: config.toleranceOperator ?? config.operators[0].value, value: "" });
  };

  const removeCondition = (index: number) => {
    onChange(conditions.filter((_, i) => i !== index));
  };

  const addCondition = () => {
    onChange([...conditions, blankConditionFor(fieldConfigs[0])]);
  };

  return (
    <Flex vertical gap="small">
      {conditions.map((condition, index) => {
        const config = configFor(condition.field);
        return (
          <Flex key={index} gap="small" align="center" className="condition-row">
            <Select
              aria-label="Campo"
              value={condition.field}
              options={fieldOptions}
              onChange={(value: RuleConditionField) => changeField(index, value)}
            />
            {config.toleranceOperator ? (
              <Input
                aria-label={`Tolerância${config.toleranceSuffix ? ` (${config.toleranceSuffix})` : ""}`}
                type="number"
                min={0}
                value={condition.value}
                onChange={(event) => updateCondition(index, { value: event.target.value })}
                placeholder="Tolerância"
                suffix={config.toleranceSuffix}
              />
            ) : (
              <>
                <Select
                  aria-label="Operador"
                  value={condition.operator}
                  options={config.operators}
                  onChange={(value: RuleConditionOperator) => updateCondition(index, { operator: value })}
                />
                {config.valueOptions ? (
                  <Select
                    aria-label="Valor"
                    value={condition.value || undefined}
                    options={config.valueOptions(condition.field)}
                    loading={config.valueOptionsLoading}
                    showSearch
                    optionFilterProp="label"
                    placeholder={config.valuePlaceholder?.(condition.field) ?? "Selecione"}
                    notFoundContent={config.valueNotFoundContent?.(condition.field)}
                    onChange={(value: string) => updateCondition(index, { value })}
                  />
                ) : (
                  <Input
                    aria-label="Valor"
                    value={condition.value}
                    onChange={(event) => updateCondition(index, { value: event.target.value })}
                    placeholder="Valor"
                  />
                )}
              </>
            )}
            <Button
              aria-label="Remover condição"
              icon={<DeleteOutlined aria-hidden="true" />}
              disabled={conditions.length <= minConditions}
              onClick={() => removeCondition(index)}
            />
          </Flex>
        );
      })}
      <Button icon={<PlusOutlined aria-hidden="true" />} onClick={addCondition}>
        Adicionar condição
      </Button>
    </Flex>
  );
}
