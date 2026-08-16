import {
  parseRecurringCommitment,
  parseRecurringCommitmentList,
  parseProblem,
  type Problem,
  type RecurringCommitment,
  type RecurringCommitmentWrite,
} from "./contracts";
import { ApiError, isAbortError } from "./problems";

const defaultMessage = "Não foi possível salvar o compromisso recorrente.";

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
      response.status === 404 ? "not_found" : "response",
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

export function listRecurringCommitments(signal?: AbortSignal): Promise<RecurringCommitment[]> {
  return send(
    "/api/recurring-commitments",
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseRecurringCommitmentList,
  );
}

export function createRecurringCommitment(
  write: RecurringCommitmentWrite,
): Promise<RecurringCommitment> {
  return send(
    "/api/recurring-commitments",
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    201,
    parseRecurringCommitment,
  );
}

export function updateRecurringCommitment(
  commitmentId: string,
  write: RecurringCommitmentWrite,
): Promise<RecurringCommitment> {
  return send(
    `/api/recurring-commitments/${encodeURIComponent(commitmentId)}`,
    {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    200,
    parseRecurringCommitment,
  );
}

export function setRecurringCommitmentActive(
  commitmentId: string,
  isActive: boolean,
): Promise<RecurringCommitment> {
  return send(
    `/api/recurring-commitments/${encodeURIComponent(commitmentId)}`,
    {
      method: "PATCH",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ is_active: isActive }),
    },
    200,
    parseRecurringCommitment,
  );
}

export function deleteRecurringCommitment(commitmentId: string): Promise<void> {
  return send(
    `/api/recurring-commitments/${encodeURIComponent(commitmentId)}`,
    { method: "DELETE" },
    204,
    null,
  );
}
