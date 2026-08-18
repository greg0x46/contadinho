import type { RuleCondition, RuleConditionField, RuleConditionOperator, RuleLogicOperator } from "../api/contracts";

export const ruleConditionFieldLabel: Record<RuleConditionField, string> = {
  description: "Descrição",
  card: "Cartão",
  account: "Conta",
  amount: "Valor",
  day_of_month: "Dia do mês",
};

export const ruleConditionOperatorLabel: Record<RuleConditionOperator, string> = {
  contains: "contém",
  equals: "é",
  within_percent: "próximo de",
  day_range: "entre os dias",
};

export const ruleLogicOperatorLabel: Record<RuleLogicOperator, string> = {
  and: "E",
  or: "OU",
};

function formatConditionValue(condition: RuleCondition): string {
  if (condition.operator === "day_range") {
    const [min, max] = condition.value.split(":");
    return `${min} e ${max}`;
  }
  if (condition.operator === "within_percent") {
    const [reference, tolerance] = condition.value.split(":");
    return `${reference} (±${tolerance}%)`;
  }
  return `"${condition.value}"`;
}

export function summarizeConditions(conditions: RuleCondition[], logicOperator: RuleLogicOperator): string {
  return conditions
    .map(
      (condition) =>
        `${ruleConditionFieldLabel[condition.field]} ${ruleConditionOperatorLabel[condition.operator]} ${formatConditionValue(condition)}`,
    )
    .join(` ${ruleLogicOperatorLabel[logicOperator]} `);
}
