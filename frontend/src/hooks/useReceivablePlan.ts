import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createReceivableScenario,
  createRealization,
  deleteRealization,
  deleteScenarioTransaction,
  generateInstallments,
  getScenario,
  listReceivableScenarios,
  readjustInstallments,
} from "../api/scenarios";
import type {
  GenerateInstallmentsWrite,
  ReadjustWrite,
  RealizationWrite,
  ScenarioDetail,
} from "../api/contracts";

export const receivableScenariosQueryKey = (receivableId: string) =>
  ["receivables", receivableId, "scenarios"] as const;

// useReceivablePlan mirrors useDebtPlan: it surfaces the single
// receivable_plan Scenario for a receivable, treating the first one found
// as *the* plan, loading its transactions via getScenario.
export function useReceivablePlan(receivableId: string) {
  const queryClient = useQueryClient();

  const listQuery = useQuery({
    queryKey: receivableScenariosQueryKey(receivableId),
    queryFn: ({ signal }) => listReceivableScenarios(receivableId, signal),
  });
  const planSummary = listQuery.data?.[0] ?? null;

  const detailQuery = useQuery({
    queryKey: [...receivableScenariosQueryKey(receivableId), planSummary?.id ?? null, "detail"],
    queryFn: ({ signal }) => getScenario(planSummary!.id, signal),
    enabled: planSummary !== null,
  });

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: receivableScenariosQueryKey(receivableId) });

  const createMutation = useMutation({
    mutationFn: (name: string) => createReceivableScenario(receivableId, { name }),
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

  const deallocateMutation = useMutation({
    mutationFn: ({ transactionId, realizationId }: { transactionId: string; realizationId: string }) => {
      if (planSummary === null) throw new Error("Nenhum plano ativo.");
      return deleteRealization(planSummary.id, transactionId, realizationId);
    },
    onSuccess: invalidate,
  });

  const readjustMutation = useMutation({
    mutationFn: (write: ReadjustWrite) => {
      if (planSummary === null) throw new Error("Nenhum plano ativo.");
      return readjustInstallments(planSummary.id, write);
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
    deallocateRealization: deallocateMutation.mutateAsync,
    isDeallocating: deallocateMutation.isPending,
    readjust: readjustMutation.mutateAsync,
    isReadjusting: readjustMutation.isPending,
  };
}
