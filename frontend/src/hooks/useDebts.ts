import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { createDebt, deleteDebt, listDebts, updateDebt } from "../api/debts";
import type { DebtCreate, DebtUpdate } from "../api/contracts";

export const debtsQueryKey = ["debts"] as const;

export function useDebts() {
  const queryClient = useQueryClient();
  const debtsQuery = useQuery({
    queryKey: debtsQueryKey,
    queryFn: ({ signal }) => listDebts(signal),
  });

  const createMutation = useMutation({
    mutationFn: (write: DebtCreate) => createDebt(write),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: debtsQueryKey }),
  });

  const updateMutation = useMutation({
    mutationFn: ({ debtId, write }: { debtId: string; write: DebtUpdate }) =>
      updateDebt(debtId, write),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: debtsQueryKey }),
  });

  const deleteMutation = useMutation({
    mutationFn: (debtId: string) => deleteDebt(debtId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: debtsQueryKey }),
  });

  return {
    debts: debtsQuery.data ?? [],
    isLoading: debtsQuery.isLoading,
    error: debtsQuery.error,
    refetch: () => debtsQuery.refetch(),
    createDebt: createMutation.mutateAsync,
    isSaving: createMutation.isPending,
    updateDebt: updateMutation.mutateAsync,
    isUpdating: updateMutation.isPending,
    deleteDebt: deleteMutation.mutateAsync,
    isDeleting: deleteMutation.isPending,
  };
}
