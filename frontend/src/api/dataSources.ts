import {
  parseDataSource,
  parseDataSourceList,
  type DataSource,
} from "./contracts";
import { requestJson } from "./client";

export function listDataSources(signal?: AbortSignal): Promise<DataSource[]> {
  return requestJson(
    "/api/data-sources",
    { method: "GET", headers: { Accept: "application/json" }, signal },
    parseDataSourceList,
    200,
  );
}

export function createDataSource(
  input: { external_item_id: string; label: string | null },
  signal?: AbortSignal,
): Promise<DataSource> {
  return requestJson(
    "/api/data-sources",
    {
      method: "POST",
      headers: { Accept: "application/json", "content-type": "application/json" },
      body: JSON.stringify(input),
      signal,
    },
    parseDataSource,
    201,
  );
}

/**
 * Omitted fields are left untouched by the server, so renaming a connection
 * never has to restate whether it is active.
 */
export function updateDataSource(
  id: string,
  changes: { label?: string; is_active?: boolean },
  signal?: AbortSignal,
): Promise<DataSource> {
  return requestJson(
    `/api/data-sources/${encodeURIComponent(id)}`,
    {
      method: "PATCH",
      headers: { Accept: "application/json", "content-type": "application/json" },
      body: JSON.stringify(changes),
      signal,
    },
    parseDataSource,
    200,
  );
}
