import { describe, expect, it } from "vitest";

import type { RuleCondition } from "../api/contracts";
import { summarizeConditions } from "./ruleConditionLabels";

const condition = (field: RuleCondition["field"], operator: RuleCondition["operator"], value: string): RuleCondition =>
  ({ field, operator, value }) as RuleCondition;

describe("summarizeConditions", () => {
  it("writes a tolerance and a day range in full", () => {
    expect(summarizeConditions([condition("amount", "within_percent", "100:5")], "and")).toBe("Valor próximo de 100 (±5%)");
    expect(summarizeConditions([condition("day_of_month", "day_range", "5:10")], "and")).toBe(
      "Dia do mês entre os dias 5 e 10",
    );
  });

  it("never prints undefined for a half-filled condition", () => {
    const text = summarizeConditions(
      [condition("amount", "within_percent", "100"), condition("day_of_month", "day_range", "5")],
      "or",
    );
    expect(text).not.toContain("undefined");
    expect(text).toBe("Valor próximo de 100 OU Dia do mês entre os dias 5");
  });
});
