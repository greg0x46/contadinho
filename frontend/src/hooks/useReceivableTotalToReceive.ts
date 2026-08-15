import { useQuery } from "@tanstack/react-query";

import { getPayableTotalToReceive } from "../api/payables";

export const receivableTotalToReceiveQueryKey = ["payables", "total-to-receive"] as const;

export function useReceivableTotalToReceive() {
  const query = useQuery({
    queryKey: receivableTotalToReceiveQueryKey,
    queryFn: ({ signal }) => getPayableTotalToReceive(signal),
  });

  return {
    total: query.data,
    isLoading: query.isLoading,
    error: query.error,
    refetch: () => query.refetch(),
  };
}
