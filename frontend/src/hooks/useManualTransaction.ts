import { useMutation, useQueryClient } from "@tanstack/react-query";

import { createManualTransaction, deleteManualTransaction, updateManualTransaction } from "../api/transactions";
import type { ManualTransactionWrite } from "../api/contracts";

/**
 * Wraps the three lançamento-manual write endpoints, invalidating every
 * ["transactions", ...] query on success so the ledger, totals, and the
 * detail drawer all pick up the change — the same query key
 * useTransactionInclusion/useTransactionCategory already invalidate.
 */
export function useManualTransaction() {
  const queryClient = useQueryClient();
  const invalidateAll = () => queryClient.invalidateQueries({ queryKey: ["transactions"] });

  const createMutation = useMutation({
    mutationFn: (write: ManualTransactionWrite) => createManualTransaction(write),
    onSuccess: invalidateAll,
  });

  const updateMutation = useMutation({
    mutationFn: ({ transactionId, write }: { transactionId: string; write: ManualTransactionWrite }) =>
      updateManualTransaction(transactionId, write),
    onSuccess: invalidateAll,
  });

  const deleteMutation = useMutation({
    mutationFn: (transactionId: string) => deleteManualTransaction(transactionId),
    onSuccess: invalidateAll,
  });

  return {
    create: createMutation.mutateAsync,
    isCreating: createMutation.isPending,
    update: updateMutation.mutateAsync,
    isUpdating: updateMutation.isPending,
    remove: deleteMutation.mutateAsync,
    isRemoving: deleteMutation.isPending,
  };
}
