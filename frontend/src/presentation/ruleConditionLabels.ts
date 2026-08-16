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
  within_percent: "tolerância de %",
  near_day: "tolerância de dias",
};

export const ruleLogicOperatorLabel: Record<RuleLogicOperator, string> = {
  and: "E",
  or: "OU",
};

export function summarizeConditions(conditions: RuleCondition[], logicOperator: RuleLogicOperator): string {
  return conditions
    .map(
      (condition) =>
        `${ruleConditionFieldLabel[condition.field]} ${ruleConditionOperatorLabel[condition.operator]} "${condition.value}"`,
    )
    .join(` ${ruleLogicOperatorLabel[logicOperator]} `);
}
