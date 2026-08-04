import { useQuery } from "@tanstack/react-query";

import { getInvestment, listInvestmentTransactions } from "../api/investments";
import { ApiError } from "../api/problems";
import type { Investment } from "../api/contracts";
import { investmentsQueryKey } from "./useInvestments";

export type InvestmentDetailState = {
  investmentId: string;
  snapshot: Investment | null;
  freshness: "loading" | "fresh" | "stale" | "not_found" | "unavailable";
  retrying: boolean;
};

export function useInvestmentDetail(investmentId: string) {
  const detailQuery = useQuery({
    queryKey: [...investmentsQueryKey, investmentId],
    queryFn: ({ signal }) => getInvestment(investmentId, signal),
  });
  const snapshot = detailQuery.data ?? null;
  const freshness: InvestmentDetailState["freshness"] = detailQuery.isPending
    ? "loading"
    : detailQuery.isError
      ? snapshot !== null
        ? "stale"
        : detailQuery.error instanceof ApiError && detailQuery.error.kind === "not_found"
          ? "not_found"
          : "unavailable"
      : "fresh";
  const state: InvestmentDetailState = {
    investmentId,
    snapshot,
    freshness,
    retrying: detailQuery.isFetching && snapshot !== null,
  };
  const retry = () => void detailQuery.refetch({ cancelRefetch: true });

  const transactionsQuery = useQuery({
    queryKey: [...investmentsQueryKey, investmentId, "transactions"],
    queryFn: ({ signal }) => listInvestmentTransactions(investmentId, signal),
  });

  return {
    state,
    retry,
    transactions: transactionsQuery.data ?? [],
    transactionsLoading: transactionsQuery.isLoading,
    transactionsError: transactionsQuery.error,
    retryTransactions: () => void transactionsQuery.refetch({ cancelRefetch: true }),
  };
}
