import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { createReceivable, deleteReceivable, listReceivables, updateReceivable } from "../api/receivables";
import type { ReceivableCreate, ReceivableUpdate } from "../api/contracts";

export const receivablesQueryKey = ["receivables"] as const;

export function useReceivables() {
  const queryClient = useQueryClient();
  const receivablesQuery = useQuery({
    queryKey: receivablesQueryKey,
    queryFn: ({ signal }) => listReceivables(signal),
  });

  const createMutation = useMutation({
    mutationFn: (write: ReceivableCreate) => createReceivable(write),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: receivablesQueryKey }),
  });

  const updateMutation = useMutation({
    mutationFn: ({ receivableId, write }: { receivableId: string; write: ReceivableUpdate }) =>
      updateReceivable(receivableId, write),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: receivablesQueryKey }),
  });

  const deleteMutation = useMutation({
    mutationFn: (receivableId: string) => deleteReceivable(receivableId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: receivablesQueryKey }),
  });

  return {
    receivables: receivablesQuery.data ?? [],
    isLoading: receivablesQuery.isLoading,
    error: receivablesQuery.error,
    refetch: () => receivablesQuery.refetch(),
    createReceivable: createMutation.mutateAsync,
    isSaving: createMutation.isPending,
    updateReceivable: updateMutation.mutateAsync,
    isUpdating: updateMutation.isPending,
    deleteReceivable: deleteMutation.mutateAsync,
    isDeleting: deleteMutation.isPending,
  };
}
