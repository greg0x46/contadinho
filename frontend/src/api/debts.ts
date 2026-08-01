import {
  parseDebt,
  parseDebtDetail,
  parseDebtLink,
  parseDebtList,
  parseDebtTotalOwed,
  parseEligibleTransactionList,
  parseProblem,
  type Debt,
  type DebtCreate,
  type DebtDetail,
  type DebtLink,
  type DebtTotalOwed,
  type DebtUpdate,
  type EligibleTransaction,
  type Problem,
} from "./contracts";
import { ApiError, isAbortError } from "./problems";

const defaultMessage = "Não foi possível salvar a dívida.";

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

export function listDebts(signal?: AbortSignal): Promise<Debt[]> {
  return send(
    "/api/debts",
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseDebtList,
  );
}

export function getDebtTotalOwed(signal?: AbortSignal): Promise<DebtTotalOwed> {
  return send(
    "/api/debts/total-owed",
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseDebtTotalOwed,
  );
}

export function createDebt(write: DebtCreate): Promise<Debt> {
  return send(
    "/api/debts",
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    201,
    parseDebt,
  );
}

export function getDebt(debtId: string, signal?: AbortSignal): Promise<DebtDetail> {
  return send(
    `/api/debts/${encodeURIComponent(debtId)}`,
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseDebtDetail,
  );
}

export function updateDebt(debtId: string, write: DebtUpdate): Promise<Debt> {
  return send(
    `/api/debts/${encodeURIComponent(debtId)}`,
    {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    200,
    parseDebt,
  );
}

export function deleteDebt(debtId: string): Promise<void> {
  return send(`/api/debts/${encodeURIComponent(debtId)}`, { method: "DELETE" }, 204, null);
}

export function listEligibleTransactions(
  search: string,
  signal?: AbortSignal,
): Promise<EligibleTransaction[]> {
  const params = new URLSearchParams();
  if (search.trim() !== "") params.set("search", search.trim());
  return send(
    `/api/debts/eligible-transactions?${params.toString()}`,
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseEligibleTransactionList,
  );
}

export function createDebtLink(debtId: string, transactionId: string): Promise<DebtLink> {
  return send(
    `/api/debts/${encodeURIComponent(debtId)}/links`,
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ transaction_id: transactionId }),
    },
    201,
    parseDebtLink,
  );
}

export function deleteDebtLink(debtId: string, linkId: string): Promise<void> {
  return send(
    `/api/debts/${encodeURIComponent(debtId)}/links/${encodeURIComponent(linkId)}`,
    { method: "DELETE" },
    204,
    null,
  );
}
