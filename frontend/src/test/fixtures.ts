import type { DataSource, SyncRun, SyncRunDetail } from "../api/contracts";

export const runId = "11111111-1111-4111-8111-111111111111";
export const sourceId = "22222222-2222-4222-8222-222222222222";

export const dataSource: DataSource = {
  id: sourceId,
  provider: "pluggy",
  external_item_id: "item-1",
  display_name: "Banco de Exemplo",
  label: "Conta pessoal",
  name: "Conta pessoal",
  is_active: true,
  created_at: "2026-07-29T12:00:00Z",
  updated_at: "2026-07-29T12:00:00Z",
};

export const syncRun: SyncRun = {
  id: runId,
  status: "in_progress",
  source_id: sourceId,
  source_name: "Conta pessoal",
  started_at: "2026-07-29T12:00:00Z",
  finished_at: null,
  accounts_processed: 2,
  transactions_inserted: 3,
  transactions_updated: 4,
  result_message: "Synchronization is in progress.",
};

export const syncRunDetail: SyncRunDetail = { ...syncRun, failures: [] };

export const jsonResponse = (body: unknown, init: ResponseInit = {}) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { "content-type": "application/json" },
    ...init,
  });
