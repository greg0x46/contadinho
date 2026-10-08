import { useQuery } from "@tanstack/react-query";

import { getNetWorth } from "../api/netWorth";
import { queryKeys } from "../api/queryKeys";

export function useNetWorth() {
  const query = useQuery({
    queryKey: queryKeys.netWorth,
    queryFn: ({ signal }) => getNetWorth(signal),
  });

  return {
    series: query.data?.series ?? [],
    latest: query.data?.latest ?? null,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}
