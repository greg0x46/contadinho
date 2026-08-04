import {
  parseEligibleTransactionList,
  parseProblem,
  parseReceivable,
  parseReceivableDetail,
  parseReceivableLink,
  parseReceivableList,
  parseReceivableTotalToReceive,
  type EligibleTransaction,
  type Problem,
  type Receivable,
  type ReceivableCreate,
  type ReceivableDetail,
  type ReceivableLink,
  type ReceivableTotalToReceive,
  type ReceivableUpdate,
} from "./contracts";
import { ApiError, isAbortError } from "./problems";

const defaultMessage = "Não foi possível salvar a conta a receber.";

async function send<T>(
  input: RequestInfo | URL,
  init: RequestInit,
  expectedStatus: number,
  parser: ((value: unknown) => T) | null,
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(input, init);
  } catch (error) {
    if (isAbortError(error)) throw error;
    throw new ApiError("transport", defaultMessage);
  }

  const contentType = response.headers.get("content-type")?.split(";")[0].trim();

  if (response.status !== expectedStatus) {
    let problem: Problem | undefined;
    if (contentType === "application/problem+json") {
      try {
        problem = parseProblem(await response.json());
      } catch {
        problem = undefined;
      }
    }
    throw new ApiError(
      response.status === 404 ? "not_found" : response.status === 409 ? "conflict" : "response",
      problem?.detail ?? problem?.title ?? defaultMessage,
      problem,
    );
  }

  if (parser === null) {
    return undefined as T;
  }
  if (contentType !== "application/json") {
    throw new ApiError("response", defaultMessage);
  }
  try {
    return parser(await response.json());
  } catch {
    throw new ApiError("response", defaultMessage);
  }
}

export function listReceivables(signal?: AbortSignal): Promise<Receivable[]> {
  return send(
    "/api/receivables",
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseReceivableList,
  );
}

export function getReceivableTotalToReceive(signal?: AbortSignal): Promise<ReceivableTotalToReceive> {
  return send(
    "/api/receivables/total-to-receive",
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseReceivableTotalToReceive,
  );
}

export function createReceivable(write: ReceivableCreate): Promise<Receivable> {
  return send(
    "/api/receivables",
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    201,
    parseReceivable,
  );
}

export function getReceivable(receivableId: string, signal?: AbortSignal): Promise<ReceivableDetail> {
  return send(
    `/api/receivables/${encodeURIComponent(receivableId)}`,
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseReceivableDetail,
  );
}

export function updateReceivable(receivableId: string, write: ReceivableUpdate): Promise<Receivable> {
  return send(
    `/api/receivables/${encodeURIComponent(receivableId)}`,
    {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    200,
    parseReceivable,
  );
}

export function deleteReceivable(receivableId: string): Promise<void> {
  return send(`/api/receivables/${encodeURIComponent(receivableId)}`, { method: "DELETE" }, 204, null);
}

export function listEligibleReceivableTransactions(
  search: string,
  signal?: AbortSignal,
): Promise<EligibleTransaction[]> {
  const params = new URLSearchParams();
  if (search.trim() !== "") params.set("search", search.trim());
  return send(
    `/api/receivables/eligible-transactions?${params.toString()}`,
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseEligibleTransactionList,
  );
}

export function createReceivableLink(receivableId: string, transactionId: string): Promise<ReceivableLink> {
  return send(
    `/api/receivables/${encodeURIComponent(receivableId)}/links`,
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ transaction_id: transactionId }),
    },
    201,
    parseReceivableLink,
  );
}

export function deleteReceivableLink(receivableId: string, linkId: string): Promise<void> {
  return send(
    `/api/receivables/${encodeURIComponent(receivableId)}/links/${encodeURIComponent(linkId)}`,
    { method: "DELETE" },
    204,
    null,
  );
}
