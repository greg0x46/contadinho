import {
  parseSyncRun,
  parseSyncRunDetail,
  parseSyncRunList,
  type SyncRun,
  type SyncRunDetail,
} from "./contracts";
import { requestJson } from "./client";

export function createSyncRun(signal?: AbortSignal): Promise<SyncRun> {
  return requestJson(
    "/api/sync-runs",
    { method: "POST", headers: { Accept: "application/json" }, signal },
    parseSyncRun,
    202,
  );
}

export function getSyncRun(id: string, signal?: AbortSignal): Promise<SyncRunDetail> {
  return requestJson(
    `/api/sync-runs/${encodeURIComponent(id)}`,
    { method: "GET", headers: { Accept: "application/json" }, signal },
    parseSyncRunDetail,
    200,
  );
}

export function listSyncRuns(signal?: AbortSignal): Promise<SyncRun[]> {
  return requestJson(
    "/api/sync-runs?limit=20",
    { method: "GET", headers: { Accept: "application/json" }, signal },
    parseSyncRunList,
    200,
  );
}
