import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { putReconciliation } from "../api/recurringCommitments";
import { getTransactionReconciliation } from "../api/transactions";
import { recurringCommitmentsQueryKey } from "./useRecurringCommitments";

export const transactionReconciliationQueryKey = (transactionId: string) =>
  ["transactionReconciliation", transactionId] as const;

/**
 * The transaction drawer's side of reconciliation: what this transaction
 * settles, what it could settle, and the two writes.
 *
 * The writes are the same occurrence-scoped endpoints the Recorrências page
 * calls — reconciling from here is the same act seen from the other end, so
 * it must not grow a second set of rules.
 *
 * `enabled` is separate from the id so a transaction that can't reconcile
 * anything (an ignored one) skips the request while still keeping its own
 * cache entry, rather than every such transaction sharing one.
 */
export function useTransactionReconciliation(transactionId: string, enabled = true) {
  const queryClient = useQueryClient();
  const reconciliationQuery = useQuery({
    queryKey: transactionReconciliationQueryKey(transactionId),
    queryFn: ({ signal }) => getTransactionReconciliation(transactionId, signal),
    enabled,
  });

  const afterChange = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["transactionReconciliation"] }),
      queryClient.invalidateQueries({ queryKey: recurringCommitmentsQueryKey }),
      queryClient.invalidateQueries({ queryKey: ["timeline"] }),
    ]);
  };

  const reconcileMutation = useMutation({
    mutationFn: ({
      commitmentId,
      occurrenceDate,
    }: {
      commitmentId: string;
      occurrenceDate: string;
    }) =>
      putReconciliation(commitmentId, occurrenceDate, {
        state: "linked",
        transaction_id: transactionId,
      }),
    onSuccess: afterChange,
  });

  // Detaching writes the same "not reconciled" decision the Recorrências page
  // writes, and writes it whether the current match came from the rule or
  // from a previous manual pick. Merely deleting a manual link would let the
  // rule re-claim the occurrence on the very next read — the button would
  // look broken. One action, one outcome: after detaching, the occurrence is
  // not reconciled. "Voltar ao automático" is the separate, explicit undo.
  const detachMutation = useMutation({
    mutationFn: ({
      commitmentId,
      occurrenceDate,
    }: {
      commitmentId: string;
      occurrenceDate: string;
    }) => putReconciliation(commitmentId, occurrenceDate, { state: "detached" }),
    onSuccess: afterChange,
  });

  return {
    current: reconciliationQuery.data?.current ?? null,
    options: reconciliationQuery.data?.options ?? [],
    isLoading: reconciliationQuery.isLoading,
    error: reconciliationQuery.error,
    reconcile: reconcileMutation.mutateAsync,
    detach: detachMutation.mutateAsync,
    isWriting: reconcileMutation.isPending || detachMutation.isPending,
  };
}
