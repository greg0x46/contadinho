import { useQuery } from "@tanstack/react-query";

import { listAccounts } from "../api/accounts";

export const accountsQueryKey = ["accounts"] as const;

export function useAccounts() {
  const accountsQuery = useQuery({
    queryKey: accountsQueryKey,
    queryFn: ({ signal }) => listAccounts(signal),
  });

  return {
    accounts: accountsQuery.data ?? [],
    isLoading: accountsQuery.isLoading,
    error: accountsQuery.error,
    refetch: () => accountsQuery.refetch(),
  };
}
