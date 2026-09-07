import {
  parseCreateSyncRunResult,
  parseSyncRunDetail,
  parseSyncRunList,
  type CreateSyncRunResult,
  type SyncRun,
  type SyncRunDetail,
} from "./contracts";
import { requestJson } from "./client";

/**
 * Starts a run per connection and reports both the ones actually started and
 * how many were targeted (requested), so a caller can tell a partial start
 * (skipped because busy, or failed outright) apart from a complete one. With
 * no sourceId it covers every active connection; it rejects with a conflict
 * only when nothing at all could be started.
 */
export function createSyncRun(
  sourceId?: string,
  signal?: AbortSignal,
): Promise<CreateSyncRunResult> {
  return requestJson(
    "/api/sync-runs",
    {
      method: "POST",
      headers: { Accept: "application/json", "content-type": "application/json" },
      body: JSON.stringify(sourceId === undefined ? {} : { source_id: sourceId }),
      signal,
    },
    parseCreateSyncRunResult,
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
