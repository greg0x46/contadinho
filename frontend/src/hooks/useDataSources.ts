import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";

import type { DataSource } from "../api/contracts";
import { createDataSource, listDataSources, updateDataSource } from "../api/dataSources";
import { invalidateDataSourcesList, queryKeys } from "../api/queryKeys";

export type DataSourceListState =
  | { kind: "loading" }
  | { kind: "ready"; sources: DataSource[] }
  | { kind: "empty" }
  | { kind: "unavailable" };

export function useDataSources() {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: queryKeys.dataSourcesList,
    queryFn: ({ signal }) => listDataSources(signal),
  });

  const invalidate = useCallback(
    () => invalidateDataSourcesList(queryClient),
    [queryClient],
  );

  const create = useMutation({
    mutationFn: (input: { external_item_id: string; label: string | null }) =>
      createDataSource(input),
    onSuccess: invalidate,
  });
  const update = useMutation({
    mutationFn: ({ id, changes }: { id: string; changes: { label?: string; is_active?: boolean } }) =>
      updateDataSource(id, changes),
    onSuccess: invalidate,
  });

  const state: DataSourceListState = query.isPending
    ? { kind: "loading" }
    : query.isError
      ? { kind: "unavailable" }
      : query.data.length === 0
        ? { kind: "empty" }
        : { kind: "ready", sources: query.data };

  return {
    state,
    retry: () => void query.refetch(),
    create,
    update,
  };
}
