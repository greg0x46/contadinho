import { useQuery } from "@tanstack/react-query";
import { getCategoryBreakdown } from "../api/transactions";
import type { CategoryDirection } from "../api/contracts";
import { browserTimezone } from "./useTransactions";

import type { HomePeriod } from "./useHomePeriod";

export function useCategoryBreakdown(period: HomePeriod, classification: CategoryDirection) {
  const timezone = browserTimezone();
  const query = useQuery({
    queryKey: ["transactions", "category-breakdown", timezone, period, classification],
    queryFn: ({ signal }) => getCategoryBreakdown(timezone!, period, classification, signal),
    enabled: timezone !== null,
  });
  return { ...query, error: timezone === null ? new Error("Fuso horário indisponível.") : query.error };
}
