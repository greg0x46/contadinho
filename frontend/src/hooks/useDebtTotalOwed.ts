import { useQuery } from "@tanstack/react-query";

import { getPayableTotalOwed } from "../api/payables";

export const debtTotalOwedQueryKey = ["payables", "total-owed"] as const;

export function useDebtTotalOwed() {
  const query = useQuery({
    queryKey: debtTotalOwedQueryKey,
    queryFn: ({ signal }) => getPayableTotalOwed(signal),
  });

  return {
    total: query.data,
    isLoading: query.isLoading,
    error: query.error,
    refetch: () => query.refetch(),
  };
}
