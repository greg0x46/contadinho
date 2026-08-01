import {
  parseProblem,
  parseSetupStatus,
  type Problem,
  type SetupRequest,
  type SetupStatus,
  type UnlockRequest,
} from "./contracts";
import { ApiError, isAbortError } from "./problems";

const defaultMessage = "Não foi possível concluir a configuração.";

async function send(input: RequestInfo | URL, init: RequestInit, expectedStatus: number): Promise<SetupStatus> {
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
      response.status === 401
        ? "unauthorized"
        : response.status === 409
          ? "conflict"
          : "response",
      problem?.detail ?? problem?.title ?? defaultMessage,
      problem,
    );
  }

  if (contentType !== "application/json") {
    throw new ApiError("response", defaultMessage);
  }
  try {
    return parseSetupStatus(await response.json());
  } catch {
    throw new ApiError("response", defaultMessage);
  }
}

export function getSetupStatus(signal?: AbortSignal): Promise<SetupStatus> {
  return send("/api/setup/status", { method: "GET", headers: { Accept: "application/json" }, signal }, 200);
}

export function setup(payload: SetupRequest): Promise<SetupStatus> {
  return send(
    "/api/setup",
    { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(payload) },
    201,
  );
}

export function unlock(payload: UnlockRequest): Promise<SetupStatus> {
  return send(
    "/api/unlock",
    { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(payload) },
    200,
  );
}
