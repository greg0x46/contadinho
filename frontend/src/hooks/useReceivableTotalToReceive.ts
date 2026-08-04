import { useQuery } from "@tanstack/react-query";

import { getReceivableTotalToReceive } from "../api/receivables";

export const receivableTotalToReceiveQueryKey = ["receivables", "total-to-receive"] as const;

export function useReceivableTotalToReceive() {
  const query = useQuery({
    queryKey: receivableTotalToReceiveQueryKey,
    queryFn: ({ signal }) => getReceivableTotalToReceive(signal),
  });

  return {
    total: query.data,
    isLoading: query.isLoading,
    error: query.error,
    refetch: () => query.refetch(),
  };
}
