import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createScenarioTransaction,
  createStandaloneScenario,
  deleteScenario,
  deleteScenarioTransaction,
  getScenario,
  listStandaloneScenarios,
  updateScenarioTransaction,
} from "../api/scenarios";
import type { ScenarioCreate, ScenarioTransactionWrite } from "../api/contracts";

export const standaloneScenariosQueryKey = ["scenarios", "standalone"] as const;

// useScenarios is the plural counterpart to usePayablePlan (which assumes
// "the one plan of a payable"): it lists every Scenario of a given kind —
// only "standalone" has a use case today — and loads each one's detail
// (transactions) alongside it, since the UI always shows a standalone
// scenario's hypothetical transactions together with its name.
export function useScenarios({ kind }: { kind: "standalone" }) {
  const queryClient = useQueryClient();
  void kind;

  const listQuery = useQuery({
    queryKey: standaloneScenariosQueryKey,
    queryFn: ({ signal }) => listStandaloneScenarios(signal),
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: standaloneScenariosQueryKey });

  const createMutation = useMutation({
    mutationFn: (write: ScenarioCreate) => createStandaloneScenario(write),
    onSuccess: invalidate,
  });

  const deleteMutation = useMutation({
    mutationFn: (scenarioId: string) => deleteScenario(scenarioId),
    onSuccess: invalidate,
  });

  const createTransactionMutation = useMutation({
    mutationFn: ({ scenarioId, write }: { scenarioId: string; write: ScenarioTransactionWrite }) =>
      createScenarioTransaction(scenarioId, write),
    onSuccess: invalidate,
  });

  const updateTransactionMutation = useMutation({
    mutationFn: ({
      scenarioId,
      transactionId,
      write,
    }: {
      scenarioId: string;
      transactionId: string;
      write: ScenarioTransactionWrite;
    }) => updateScenarioTransaction(scenarioId, transactionId, write),
    onSuccess: invalidate,
  });

  const deleteTransactionMutation = useMutation({
    mutationFn: ({ scenarioId, transactionId }: { scenarioId: string; transactionId: string }) =>
      deleteScenarioTransaction(scenarioId, transactionId),
    onSuccess: invalidate,
  });

  return {
    scenarios: listQuery.data ?? [],
    isLoading: listQuery.isLoading,
    error: listQuery.error,
    refetch: listQuery.refetch,
    createScenario: createMutation.mutateAsync,
    isCreating: createMutation.isPending,
    deleteScenario: deleteMutation.mutateAsync,
    createTransaction: createTransactionMutation.mutateAsync,
    updateTransaction: updateTransactionMutation.mutateAsync,
    deleteTransaction: deleteTransactionMutation.mutateAsync,
    getScenarioDetail: getScenario,
  };
}
