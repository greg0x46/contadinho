import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { createPayableLink, deletePayableLink, getPayable, listEligibleTransactions } from "../api/payables";
import type { PayableDetail, PayableKind } from "../api/contracts";
import { ApiError } from "../api/problems";
import { payablesQueryKey } from "./usePayables";

const SEARCH_DEBOUNCE_MS = 300;

export type PayableDetailState = {
  payableId: string;
  snapshot: PayableDetail | null;
  freshness: "loading" | "fresh" | "stale" | "not_found" | "unavailable";
  retrying: boolean;
};

export function usePayableDetail(payableId: string, kind: PayableKind) {
  const queryClient = useQueryClient();
  const baseKey = payablesQueryKey(kind);
  const detailQuery = useQuery({
    queryKey: [...baseKey, payableId],
    queryFn: ({ signal }) => getPayable(payableId, signal),
  });
  const snapshot = detailQuery.data ?? null;
  const freshness: PayableDetailState["freshness"] = detailQuery.isPending
    ? "loading"
    : detailQuery.isError
      ? snapshot !== null
        ? "stale"
        : detailQuery.error instanceof ApiError && detailQuery.error.kind === "not_found"
          ? "not_found"
          : "unavailable"
      : "fresh";
  const state: PayableDetailState = {
    payableId,
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
    queryKey: [...baseKey, "eligible-transactions", debouncedSearch],
    queryFn: ({ signal }) => listEligibleTransactions(kind, debouncedSearch, signal),
    placeholderData: keepPreviousData,
  });

  const afterLinkChange = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: [...baseKey, payableId] }),
      queryClient.invalidateQueries({ queryKey: ["payables"] }),
    ]);
  };

  const linkMutation = useMutation({
    mutationFn: (transactionId: string) => createPayableLink(payableId, transactionId),
    onSuccess: afterLinkChange,
  });

  const unlinkMutation = useMutation({
    mutationFn: (linkId: string) => deletePayableLink(payableId, linkId),
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
