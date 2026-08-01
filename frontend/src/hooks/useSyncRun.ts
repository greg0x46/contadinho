import { useQuery } from "@tanstack/react-query";

import type { SyncRunDetail } from "../api/contracts";
import { ApiError } from "../api/problems";
import { getSyncRun } from "../api/syncRuns";
import { isFinalStatus } from "../presentation/syncStatus";

export type SyncRunState = {
  runId: string;
  snapshot: SyncRunDetail | null;
  freshness: "loading" | "fresh" | "stale" | "not_found" | "unavailable";
  retrying: boolean;
};

export function useSyncRun(runId: string) {
  const query = useQuery({
    queryKey: ["sync-runs", "detail", runId],
    queryFn: ({ signal }) => getSyncRun(runId, signal),
    refetchInterval: ({ state }) =>
      state.data !== undefined && !isFinalStatus(state.data.status) ? 3000 : false,
  });
  const snapshot = query.data ?? null;
  const freshness: SyncRunState["freshness"] = query.isPending
    ? "loading"
    : query.isError
      ? snapshot !== null
        ? "stale"
        : query.error instanceof ApiError && query.error.kind === "not_found"
          ? "not_found"
          : "unavailable"
      : "fresh";
  const state: SyncRunState = {
    runId,
    snapshot,
    freshness,
    retrying: query.isFetching && snapshot !== null,
  };
  const retry = () => void query.refetch({ cancelRefetch: true });

  return { state, retry };
}
