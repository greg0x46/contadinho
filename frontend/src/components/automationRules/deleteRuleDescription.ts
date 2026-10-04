import type { AutomationRule } from "../../api/contracts";

/** What deleting a rule leaves untouched — shown by every "Excluir automação" confirmation. */
export function deleteRuleDescription(rule: AutomationRule): string {
  return rule.actions.some((action) => action.type === "reconcile")
    ? "A recorrência vinculada não será excluída, apenas deixará de ser conciliada automaticamente. Transações já ignoradas/categorizadas por esta regra permanecem como estão."
    : "Transações já ignoradas/categorizadas por esta regra permanecem como estão.";
}
