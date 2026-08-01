import { useQuery } from "@tanstack/react-query";

import type { SyncRun } from "../api/contracts";
import { listSyncRuns } from "../api/syncRuns";

export type SyncRunListState =
  | { kind: "loading" }
  | { kind: "ready"; runs: SyncRun[] }
  | { kind: "empty" }
  | { kind: "unavailable" };

export function useSyncRunList() {
  const query = useQuery({
    queryKey: ["sync-runs", "list"],
    queryFn: ({ signal }) => listSyncRuns(signal),
  });
  const state: SyncRunListState = query.isPending
    ? { kind: "loading" }
    : query.isError
      ? { kind: "unavailable" }
      : query.data.length === 0
        ? { kind: "empty" }
        : { kind: "ready", runs: query.data };
  const retry = () => void query.refetch();

  return { state, retry };
}
