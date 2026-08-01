import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createDebtScenario,
  createRealization,
  deleteScenarioTransaction,
  generateInstallments,
  getScenario,
  listDebtScenarios,
} from "../api/scenarios";
import type { GenerateInstallmentsWrite, RealizationWrite, ScenarioDetail } from "../api/contracts";

export const debtScenariosQueryKey = (debtId: string) => ["debts", debtId, "scenarios"] as const;

// useDebtPlan surfaces the single debt_plan Scenario for a debt (task 1's
// roadmap keeps this to one plan per debt in the UI, even though the data
// model allows more): it lists the debt's scenarios, and treats the first
// one found as *the* plan, loading its transactions via getScenario.
export function useDebtPlan(debtId: string) {
  const queryClient = useQueryClient();

  const listQuery = useQuery({
    queryKey: debtScenariosQueryKey(debtId),
    queryFn: ({ signal }) => listDebtScenarios(debtId, signal),
  });
  const planSummary = listQuery.data?.[0] ?? null;

  const detailQuery = useQuery({
    queryKey: [...debtScenariosQueryKey(debtId), planSummary?.id ?? null, "detail"],
    queryFn: ({ signal }) => getScenario(planSummary!.id, signal),
    enabled: planSummary !== null,
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: debtScenariosQueryKey(debtId) });

  const createMutation = useMutation({
    mutationFn: (name: string) => createDebtScenario(debtId, { name }),
    onSuccess: invalidate,
  });

  const generateMutation = useMutation({
    mutationFn: (write: GenerateInstallmentsWrite) => {
      if (planSummary === null) throw new Error("Nenhum plano para gerar parcelas.");
      return generateInstallments(planSummary.id, write);
    },
    onSuccess: invalidate,
  });

  const deleteTransactionMutation = useMutation({
    mutationFn: (transactionId: string) => {
      if (planSummary === null) throw new Error("Nenhum plano ativo.");
      return deleteScenarioTransaction(planSummary.id, transactionId);
    },
    onSuccess: invalidate,
  });

  const allocateMutation = useMutation({
    mutationFn: ({ transactionId, write }: { transactionId: string; write: RealizationWrite }) => {
      if (planSummary === null) throw new Error("Nenhum plano ativo.");
      return createRealization(planSummary.id, transactionId, write);
    },
    onSuccess: invalidate,
  });

  const plan: ScenarioDetail | null = detailQuery.data ?? null;

  return {
    plan,
    isLoading: listQuery.isLoading || (planSummary !== null && detailQuery.isLoading),
    error: listQuery.error ?? detailQuery.error,
    createPlan: createMutation.mutateAsync,
    isCreating: createMutation.isPending,
    generateInstallments: generateMutation.mutateAsync,
    isGenerating: generateMutation.isPending,
    deleteInstallment: deleteTransactionMutation.mutateAsync,
    isDeletingInstallment: deleteTransactionMutation.isPending,
    allocateRealization: allocateMutation.mutateAsync,
    isAllocating: allocateMutation.isPending,
  };
}
