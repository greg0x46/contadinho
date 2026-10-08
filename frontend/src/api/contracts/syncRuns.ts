import { isCount, isNullableString, isRecord, isUuid, isValidDate } from "./shared";

export const syncStatuses = [
  "in_progress",
  "completed",
  "completed_with_failures",
  "failed",
] as const;

export type SyncStatus = (typeof syncStatuses)[number];

export const failureStages = [
  "auth",
  "item",
  "accounts",
  "account",
  "transactions",
  "normalize",
  "interrupted",
  "worker_unavailable",
] as const;

export type FailureStage = (typeof failureStages)[number];

export interface SyncRun {
  id: string;
  status: SyncStatus;
  /** Which connection this run refreshed — with several banks, a run means nothing without it. */
  source_id: string;
  source_name: string;
  started_at: string;
  finished_at: string | null;
  accounts_processed: number;
  transactions_inserted: number;
  transactions_updated: number;
  result_message: string | null;
}

export interface SyncFailure {
  stage: FailureStage;
  code: string;
  message: string;
  external_account_id: string | null;
  external_transaction_id: string | null;
  occurred_at: string;
}

export interface SyncRunDetail extends SyncRun {
  failures: SyncFailure[];
}

/**
 * requested counts every connection the create request targeted, which can
 * exceed runs.length: a connection already syncing, or one whose insert
 * failed outright, is skipped rather than failing the whole request, so a
 * shorter runs list is otherwise indistinguishable from "that's everything".
 */
export interface CreateSyncRunResult {
  runs: SyncRun[];
  requested: number;
}

export function parseSyncRun(value: unknown): SyncRun {
  if (
    !isRecord(value) ||
    typeof value.id !== "string" ||
    !isUuid(value.id) ||
    typeof value.source_id !== "string" ||
    !isUuid(value.source_id) ||
    typeof value.source_name !== "string" ||
    !syncStatuses.includes(value.status as SyncStatus) ||
    !isValidDate(value.started_at) ||
    !(value.finished_at === null || isValidDate(value.finished_at)) ||
    !isCount(value.accounts_processed) ||
    !isCount(value.transactions_inserted) ||
    !isCount(value.transactions_updated) ||
    !isNullableString(value.result_message)
  ) {
    throw new TypeError("Resposta de execução inválida.");
  }

  return {
    id: value.id,
    status: value.status as SyncStatus,
    source_id: value.source_id,
    source_name: value.source_name,
    started_at: value.started_at,
    finished_at: value.finished_at,
    accounts_processed: value.accounts_processed,
    transactions_inserted: value.transactions_inserted,
    transactions_updated: value.transactions_updated,
    result_message: value.result_message,
  };
}

function parseFailure(value: unknown): SyncFailure {
  if (
    !isRecord(value) ||
    !failureStages.includes(value.stage as FailureStage) ||
    typeof value.code !== "string" ||
    typeof value.message !== "string" ||
    !isNullableString(value.external_account_id) ||
    !isNullableString(value.external_transaction_id) ||
    !isValidDate(value.occurred_at)
  ) {
    throw new TypeError("Falha de sincronização inválida.");
  }

  return {
    stage: value.stage as FailureStage,
    code: value.code,
    message: value.message,
    external_account_id: value.external_account_id,
    external_transaction_id: value.external_transaction_id,
    occurred_at: value.occurred_at,
  };
}

export function parseSyncRunDetail(value: unknown): SyncRunDetail {
  if (!isRecord(value) || !Array.isArray(value.failures)) {
    throw new TypeError("Detalhe de execução inválido.");
  }
  return { ...parseSyncRun(value), failures: value.failures.map(parseFailure) };
}

export function parseSyncRunList(value: unknown): SyncRun[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de execuções inválida.");
  }
  return value.map(parseSyncRun);
}

export function parseCreateSyncRunResult(value: unknown): CreateSyncRunResult {
  if (!isRecord(value) || !Array.isArray(value.runs) || !isCount(value.requested)) {
    throw new TypeError("Resposta de criação de execução inválida.");
  }
  return { runs: value.runs.map(parseSyncRun), requested: value.requested };
}
