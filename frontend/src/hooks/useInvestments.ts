import { useQuery } from "@tanstack/react-query";

import { listInvestments } from "../api/investments";
import { queryKeys } from "../api/queryKeys";

export const investmentsQueryKey = queryKeys.investments;

export function useInvestments() {
  const investmentsQuery = useQuery({
    queryKey: investmentsQueryKey,
    queryFn: ({ signal }) => listInvestments(signal),
  });

  return {
    investments: investmentsQuery.data ?? [],
    isLoading: investmentsQuery.isLoading,
    error: investmentsQuery.error,
    refetch: () => investmentsQuery.refetch(),
  };
}
