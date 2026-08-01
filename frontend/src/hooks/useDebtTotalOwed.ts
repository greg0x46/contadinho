import { useQuery } from "@tanstack/react-query";

import { getDebtTotalOwed } from "../api/debts";

export const debtTotalOwedQueryKey = ["debts", "total-owed"] as const;

export function useDebtTotalOwed() {
  const query = useQuery({
    queryKey: debtTotalOwedQueryKey,
    queryFn: ({ signal }) => getDebtTotalOwed(signal),
  });

  return {
    total: query.data,
    isLoading: query.isLoading,
    error: query.error,
    refetch: () => query.refetch(),
  };
}
