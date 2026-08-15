import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { createPayable, deletePayable, listPayables, updatePayable } from "../api/payables";
import type { PayableCreate, PayableKind, PayableUpdate } from "../api/contracts";

export function payablesQueryKey(kind: PayableKind | null) {
  return ["payables", kind ?? "all"] as const;
}

export function usePayables(kind: PayableKind | null = null) {
  const queryClient = useQueryClient();
  const queryKey = payablesQueryKey(kind);
  const payablesQuery = useQuery({
    queryKey,
    queryFn: ({ signal }) => listPayables(kind, signal),
  });

  const invalidateAll = () => queryClient.invalidateQueries({ queryKey: ["payables"] });

  const createMutation = useMutation({
    mutationFn: (write: PayableCreate) => createPayable(write),
    onSuccess: invalidateAll,
  });

  const updateMutation = useMutation({
    mutationFn: ({ payableId, write }: { payableId: string; write: PayableUpdate }) =>
      updatePayable(payableId, write),
    onSuccess: invalidateAll,
  });

  const deleteMutation = useMutation({
    mutationFn: (payableId: string) => deletePayable(payableId),
    onSuccess: invalidateAll,
  });

  return {
    payables: payablesQuery.data ?? [],
    isLoading: payablesQuery.isLoading,
    error: payablesQuery.error,
    refetch: () => payablesQuery.refetch(),
    createPayable: createMutation.mutateAsync,
    isSaving: createMutation.isPending,
    updatePayable: updateMutation.mutateAsync,
    isUpdating: updateMutation.isPending,
    deletePayable: deleteMutation.mutateAsync,
    isDeleting: deleteMutation.isPending,
  };
}
