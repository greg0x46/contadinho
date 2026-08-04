import { useQuery } from "@tanstack/react-query";

import { listInvestments } from "../api/investments";

export const investmentsQueryKey = ["investments"] as const;

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
