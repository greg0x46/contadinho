import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import {
  deleteReconciliation,
  listReconciliationCandidates,
  listRecurrenceOccurrences,
  putReconciliation,
} from "../api/recurringCommitments";
import type { ReconciliationWrite } from "../api/contracts";
import { recurringCommitmentsQueryKey } from "./useRecurringCommitments";

const SEARCH_DEBOUNCE_MS = 300;

export const recurrenceOccurrencesQueryKey = (commitmentId: string) =>
  [...recurringCommitmentsQueryKey, commitmentId, "occurrences"] as const;

/**
 * Occurrences of one commitment plus the three writes the UI offers on them:
 * link a transaction, detach the occurrence, or forget the decision so the
 * automation rule takes over again.
 *
 * `enabled` exists because this is mounted from an expandable table row —
 * a collapsed row shouldn't fetch, and every commitment on screen expanding
 * eagerly would be one request per row.
 */
export function useRecurrenceOccurrences(commitmentId: string, enabled = true) {
  const queryClient = useQueryClient();
  const occurrencesQuery = useQuery({
    queryKey: recurrenceOccurrencesQueryKey(commitmentId),
    queryFn: ({ signal }) => listRecurrenceOccurrences(commitmentId, signal),
    enabled,
  });

  // Reconciling changes which occurrences project, so the Home's projection
  // and the transaction views that read the same decision have to be
  // refetched too.
  const afterReconciliationChange = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: recurrenceOccurrencesQueryKey(commitmentId) }),
      queryClient.invalidateQueries({ queryKey: ["timeline"] }),
      queryClient.invalidateQueries({ queryKey: ["transactions"] }),
      queryClient.invalidateQueries({ queryKey: ["transactionReconciliation"] }),
    ]);
  };

  const putMutation = useMutation({
    mutationFn: ({
      occurrenceDate,
      write,
    }: {
      occurrenceDate: string;
      write: ReconciliationWrite;
    }) => putReconciliation(commitmentId, occurrenceDate, write),
    onSuccess: afterReconciliationChange,
  });

  const deleteMutation = useMutation({
    mutationFn: (occurrenceDate: string) => deleteReconciliation(commitmentId, occurrenceDate),
    onSuccess: afterReconciliationChange,
  });

  return {
    occurrences: occurrencesQuery.data ?? [],
    isLoading: occurrencesQuery.isLoading,
    error: occurrencesQuery.error,
    retry: () => void occurrencesQuery.refetch({ cancelRefetch: true }),
    reconcile: (occurrenceDate: string, transactionId: string) =>
      putMutation.mutateAsync({
        occurrenceDate,
        write: { state: "linked", transaction_id: transactionId },
      }),
    detach: (occurrenceDate: string) =>
      putMutation.mutateAsync({ occurrenceDate, write: { state: "detached" } }),
    restoreAutomatic: deleteMutation.mutateAsync,
    isWriting: putMutation.isPending || deleteMutation.isPending,
  };
}

/**
 * Server-side candidate search for one occurrence, debounced so typing in the
 * picker doesn't fire a request per keystroke. Mirrors usePayableDetail's
 * eligible-transaction search, which solves the same problem from the
 * payable side.
 *
 * occurrenceDate is null while the picker is closed, which is what keeps a
 * closed dialog from querying.
 */
export function useReconciliationCandidates(commitmentId: string, occurrenceDate: string | null) {
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [search]);

  const candidatesQuery = useQuery({
    queryKey: [
      ...recurringCommitmentsQueryKey,
      commitmentId,
      "candidates",
      occurrenceDate,
      debouncedSearch,
    ],
    queryFn: ({ signal }) =>
      listReconciliationCandidates(commitmentId, occurrenceDate!, debouncedSearch, signal),
    enabled: occurrenceDate !== null,
    placeholderData: keepPreviousData,
  });

  return {
    search,
    setSearch,
    candidates: candidatesQuery.data ?? [],
    isSearching: candidatesQuery.isFetching,
  };
}
