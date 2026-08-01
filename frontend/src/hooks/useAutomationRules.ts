import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createAutomationRule,
  deleteAutomationRule,
  listAutomationRuleConditionOptions,
  listAutomationRules,
  setAutomationRuleActive,
  updateAutomationRule,
} from "../api/automationRules";
import type { AutomationRuleWrite, AutomationRuleWriteResult } from "../api/contracts";

export const automationRulesQueryKey = ["automationRules"] as const;
export const automationRuleConditionOptionsQueryKey = [
  "automationRuleConditionOptions",
] as const;

export function useAutomationRules() {
  const queryClient = useQueryClient();
  const rulesQuery = useQuery({
    queryKey: automationRulesQueryKey,
    queryFn: ({ signal }) => listAutomationRules(signal),
  });
  const conditionOptionsQuery = useQuery({
    queryKey: automationRuleConditionOptionsQueryKey,
    queryFn: ({ signal }) => listAutomationRuleConditionOptions(signal),
  });

  const afterWrite = async (result: AutomationRuleWriteResult) => {
    await queryClient.invalidateQueries({ queryKey: automationRulesQueryKey });
    if (result.retroactive_apply !== null) {
      await queryClient.invalidateQueries({ queryKey: ["transactions"] });
    }
  };

  const createMutation = useMutation({
    mutationFn: (write: AutomationRuleWrite) => createAutomationRule(write),
    onSuccess: afterWrite,
  });

  const updateMutation = useMutation({
    mutationFn: ({ ruleId, write }: { ruleId: string; write: AutomationRuleWrite }) =>
      updateAutomationRule(ruleId, write),
    onSuccess: afterWrite,
  });

  const toggleMutation = useMutation({
    mutationFn: ({ ruleId, isActive }: { ruleId: string; isActive: boolean }) =>
      setAutomationRuleActive(ruleId, isActive),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: automationRulesQueryKey }),
  });

  const deleteMutation = useMutation({
    mutationFn: (ruleId: string) => deleteAutomationRule(ruleId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: automationRulesQueryKey }),
  });

  return {
    rules: rulesQuery.data ?? [],
    conditionOptions: conditionOptionsQuery.data ?? { accounts: [], cards: [] },
    areConditionOptionsLoading: conditionOptionsQuery.isLoading,
    isLoading: rulesQuery.isLoading,
    error: rulesQuery.error ?? conditionOptionsQuery.error,
    refetch: async () => {
      await Promise.all([rulesQuery.refetch(), conditionOptionsQuery.refetch()]);
    },
    createRule: createMutation.mutateAsync,
    updateRule: updateMutation.mutateAsync,
    toggleRule: toggleMutation.mutateAsync,
    deleteRule: deleteMutation.mutateAsync,
    isSaving: createMutation.isPending || updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
  };
}
