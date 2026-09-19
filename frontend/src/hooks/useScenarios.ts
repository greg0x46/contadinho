import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createScenarioTransaction,
  createStandaloneScenario,
  deleteScenario,
  deleteScenarioTransaction,
  getScenario,
  listScenarios,
  listStandaloneScenarios,
  setScenarioActive,
  updateScenarioTransaction,
} from "../api/scenarios";
import type { ScenarioCreate, ScenarioKind, ScenarioTransactionWrite } from "../api/contracts";

export const standaloneScenariosQueryKey = ["scenarios", "standalone"] as const;
export const allScenariosQueryKey = ["scenarios", "all"] as const;

// useScenarios is the plural counterpart to usePayablePlan (which assumes
// "the one plan of a payable"): it lists every Scenario of a given kind —
// only "standalone" has a use case today — and loads each one's detail
// (transactions) alongside it, since the UI always shows a standalone
// scenario's hypothetical transactions together with its name.
export function useScenarios({ kind }: { kind?: ScenarioKind } = {}) {
  const queryClient = useQueryClient();
  const legacyStandaloneQuery = kind === "standalone";
  const queryKey = legacyStandaloneQuery
    ? standaloneScenariosQueryKey
    : kind === undefined
      ? allScenariosQueryKey
      : (["scenarios", kind] as const);

  const listQuery = useQuery({
    queryKey,
    queryFn: async ({ signal }) => {
      if (legacyStandaloneQuery) return listStandaloneScenarios(signal);
      const list = await listScenarios(kind === undefined ? {} : { kind }, signal);
      // Keeps old mocked/compatibility clients usable while the generic
      // endpoint rolls out. A real empty catalog is [] and never falls back.
      return list ?? listStandaloneScenarios(signal);
    },
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["scenarios"] });
    // Every mutation here — activating a scenario, editing its transactions
    // — changes what the unified projector includes, so any cached balance
    // curve or period total is now stale.
    queryClient.invalidateQueries({ queryKey: ["timeline"] });
    queryClient.invalidateQueries({ queryKey: ["timeline-data-range"] });
  };

  const createMutation = useMutation({
    mutationFn: (write: ScenarioCreate) => createStandaloneScenario(write),
    onSuccess: invalidate,
  });

  const deleteMutation = useMutation({
    mutationFn: (scenarioId: string) => deleteScenario(scenarioId),
    onSuccess: invalidate,
  });

  const toggleMutation = useMutation({
    mutationFn: ({ scenarioId, isActive }: { scenarioId: string; isActive: boolean }) =>
      setScenarioActive(scenarioId, isActive),
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
    toggleScenario: toggleMutation.mutateAsync,
    createTransaction: createTransactionMutation.mutateAsync,
    updateTransaction: updateTransactionMutation.mutateAsync,
    deleteTransaction: deleteTransactionMutation.mutateAsync,
    getScenarioDetail: getScenario,
  };
}
