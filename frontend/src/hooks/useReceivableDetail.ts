import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import {
  createReceivableLink,
  deleteReceivableLink,
  getReceivable,
  listEligibleReceivableTransactions,
} from "../api/receivables";
import type { ReceivableDetail } from "../api/contracts";
import { ApiError } from "../api/problems";
import { receivablesQueryKey } from "./useReceivables";

const SEARCH_DEBOUNCE_MS = 300;

export type ReceivableDetailState = {
  receivableId: string;
  snapshot: ReceivableDetail | null;
  freshness: "loading" | "fresh" | "stale" | "not_found" | "unavailable";
  retrying: boolean;
};

export function useReceivableDetail(receivableId: string) {
  const queryClient = useQueryClient();
  const detailQuery = useQuery({
    queryKey: [...receivablesQueryKey, receivableId],
    queryFn: ({ signal }) => getReceivable(receivableId, signal),
  });
  const snapshot = detailQuery.data ?? null;
  const freshness: ReceivableDetailState["freshness"] = detailQuery.isPending
    ? "loading"
    : detailQuery.isError
      ? snapshot !== null
        ? "stale"
        : detailQuery.error instanceof ApiError && detailQuery.error.kind === "not_found"
          ? "not_found"
          : "unavailable"
      : "fresh";
  const state: ReceivableDetailState = {
    receivableId,
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
    queryKey: [...receivablesQueryKey, "eligible-transactions", debouncedSearch],
    queryFn: ({ signal }) => listEligibleReceivableTransactions(debouncedSearch, signal),
    placeholderData: keepPreviousData,
  });

  const afterLinkChange = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: [...receivablesQueryKey, receivableId] }),
      queryClient.invalidateQueries({ queryKey: receivablesQueryKey }),
    ]);
  };

  const linkMutation = useMutation({
    mutationFn: (transactionId: string) => createReceivableLink(receivableId, transactionId),
    onSuccess: afterLinkChange,
  });

  const unlinkMutation = useMutation({
    mutationFn: (linkId: string) => deleteReceivableLink(receivableId, linkId),
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
