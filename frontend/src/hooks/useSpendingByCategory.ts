import { useQuery } from "@tanstack/react-query";

import { getSpendingByCategory } from "../api/transactions";
import { browserTimezone } from "./useTransactions";

export const spendingByCategoryQueryKey = (timezone: string | null) =>
  ["transactions", "spending-by-category", timezone] as const;

export function useSpendingByCategory() {
  const timezone = browserTimezone();

  const query = useQuery({
    queryKey: spendingByCategoryQueryKey(timezone),
    queryFn: ({ signal }) => getSpendingByCategory(timezone as string, signal),
    enabled: timezone !== null,
  });

  return {
    spending: query.data,
    isLoading: timezone !== null && query.isLoading,
    error: timezone === null ? new Error("Fuso horário indisponível.") : query.error,
    refetch: () => query.refetch(),
  };
}
