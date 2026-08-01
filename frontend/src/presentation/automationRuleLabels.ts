import type {
  AutomationConditionField,
  AutomationConditionOperator,
  AutomationLogicOperator,
  AutomationRuleCondition,
} from "../api/contracts";

export const conditionFieldLabel: Record<AutomationConditionField, string> = {
  description: "Descrição",
  card: "Cartão",
  account: "Conta",
};

export const conditionOperatorLabel: Record<AutomationConditionOperator, string> = {
  contains: "contém",
  equals: "é",
};

export const logicOperatorLabel: Record<AutomationLogicOperator, string> = {
  and: "E",
  or: "OU",
};

export function summarizeConditions(
  conditions: AutomationRuleCondition[],
  logicOperator: AutomationLogicOperator,
): string {
  return conditions
    .map(
      (condition) =>
        `${conditionFieldLabel[condition.field]} ${conditionOperatorLabel[condition.operator]} "${condition.value}"`,
    )
    .join(` ${logicOperatorLabel[logicOperator]} `);
}
