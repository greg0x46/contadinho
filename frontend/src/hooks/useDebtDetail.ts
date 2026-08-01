import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { createDebtLink, deleteDebtLink, getDebt, listEligibleTransactions } from "../api/debts";
import type { DebtDetail } from "../api/contracts";
import { ApiError } from "../api/problems";
import { debtsQueryKey } from "./useDebts";

const SEARCH_DEBOUNCE_MS = 300;

export type DebtDetailState = {
  debtId: string;
  snapshot: DebtDetail | null;
  freshness: "loading" | "fresh" | "stale" | "not_found" | "unavailable";
  retrying: boolean;
};

export function useDebtDetail(debtId: string) {
  const queryClient = useQueryClient();
  const detailQuery = useQuery({
    queryKey: [...debtsQueryKey, debtId],
    queryFn: ({ signal }) => getDebt(debtId, signal),
  });
  const snapshot = detailQuery.data ?? null;
  const freshness: DebtDetailState["freshness"] = detailQuery.isPending
    ? "loading"
    : detailQuery.isError
      ? snapshot !== null
        ? "stale"
        : detailQuery.error instanceof ApiError && detailQuery.error.kind === "not_found"
          ? "not_found"
          : "unavailable"
      : "fresh";
  const state: DebtDetailState = {
    debtId,
    snapshot,
    freshness,
    retrying: detailQuery.isFetching && snapshot !== null,
  };
  const retry = () => void detailQuery.refetch({ cancelRefetch: true });

  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [search]);

  const eligibleQuery = useQuery({
    queryKey: [...debtsQueryKey, "eligible-transactions", debouncedSearch],
    queryFn: ({ signal }) => listEligibleTransactions(debouncedSearch, signal),
    placeholderData: keepPreviousData,
  });

  const afterLinkChange = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: [...debtsQueryKey, debtId] }),
      queryClient.invalidateQueries({ queryKey: debtsQueryKey }),
    ]);
  };

  const linkMutation = useMutation({
    mutationFn: (transactionId: string) => createDebtLink(debtId, transactionId),
    onSuccess: afterLinkChange,
  });

  const unlinkMutation = useMutation({
    mutationFn: (linkId: string) => deleteDebtLink(debtId, linkId),
    onSuccess: afterLinkChange,
  });

  return {
    state,
    retry,
    search,
    setSearch,
    eligibleTransactions: eligibleQuery.data ?? [],
    isSearching: eligibleQuery.isFetching,
    linkTransaction: linkMutation.mutateAsync,
    isLinking: linkMutation.isPending,
    unlinkTransaction: unlinkMutation.mutateAsync,
    isUnlinking: unlinkMutation.isPending,
  };
}
