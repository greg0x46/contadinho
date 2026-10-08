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
import { invalidateAfterAutomationRuleChange, invalidateAutomationRules, queryKeys } from "../api/queryKeys";

export const automationRulesQueryKey = queryKeys.automationRules;
export const automationRuleConditionOptionsQueryKey = queryKeys.automationRuleConditionOptions;

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
    await invalidateAfterAutomationRuleChange(queryClient, {
      transactionsChanged: result.retroactive_apply !== null,
    });
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
    onSuccess: () => invalidateAutomationRules(queryClient),
  });

  const deleteMutation = useMutation({
    mutationFn: (ruleId: string) => deleteAutomationRule(ruleId),
    onSuccess: () => invalidateAutomationRules(queryClient),
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
