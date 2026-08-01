import {
  parseProblem,
  parseScenarioDetail,
  parseScenarioList,
  parseScenarioTransaction,
  type GenerateInstallmentsWrite,
  type Problem,
  type ReadjustWrite,
  type RealizationWrite,
  type Scenario,
  type ScenarioCreate,
  type ScenarioDetail,
  type ScenarioTransaction,
  type ScenarioTransactionWrite,
} from "./contracts";
import { ApiError, isAbortError } from "./problems";

const defaultMessage = "Não foi possível salvar o plano de pagamento.";

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

export function listDebtScenarios(debtId: string, signal?: AbortSignal): Promise<Scenario[]> {
  return send(
    `/api/debts/${encodeURIComponent(debtId)}/scenarios`,
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseScenarioList,
  );
}

export function createDebtScenario(debtId: string, write: ScenarioCreate): Promise<ScenarioDetail> {
  return send(
    `/api/debts/${encodeURIComponent(debtId)}/scenarios`,
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    201,
    parseScenarioDetail,
  );
}

export function getScenario(scenarioId: string, signal?: AbortSignal): Promise<ScenarioDetail> {
  return send(
    `/api/scenarios/${encodeURIComponent(scenarioId)}`,
    { method: "GET", headers: { Accept: "application/json" }, signal },
    200,
    parseScenarioDetail,
  );
}

export function deleteScenario(scenarioId: string): Promise<void> {
  return send(`/api/scenarios/${encodeURIComponent(scenarioId)}`, { method: "DELETE" }, 204, null);
}

export function createScenarioTransaction(
  scenarioId: string,
  write: ScenarioTransactionWrite,
): Promise<ScenarioTransaction> {
  return send(
    `/api/scenarios/${encodeURIComponent(scenarioId)}/transactions`,
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    201,
    parseScenarioTransaction,
  );
}

export function updateScenarioTransaction(
  scenarioId: string,
  transactionId: string,
  write: ScenarioTransactionWrite,
): Promise<ScenarioTransaction> {
  return send(
    `/api/scenarios/${encodeURIComponent(scenarioId)}/transactions/${encodeURIComponent(transactionId)}`,
    {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    200,
    parseScenarioTransaction,
  );
}

export function deleteScenarioTransaction(scenarioId: string, transactionId: string): Promise<void> {
  return send(
    `/api/scenarios/${encodeURIComponent(scenarioId)}/transactions/${encodeURIComponent(transactionId)}`,
    { method: "DELETE" },
    204,
    null,
  );
}

export function createRealization(
  scenarioId: string,
  transactionId: string,
  write: RealizationWrite,
): Promise<ScenarioTransaction> {
  return send(
    `/api/scenarios/${encodeURIComponent(scenarioId)}/transactions/${encodeURIComponent(transactionId)}/realizations`,
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    201,
    parseScenarioTransaction,
  );
}

export function deleteRealization(
  scenarioId: string,
  transactionId: string,
  realizationId: string,
): Promise<void> {
  return send(
    `/api/scenarios/${encodeURIComponent(scenarioId)}/transactions/${encodeURIComponent(transactionId)}/realizations/${encodeURIComponent(realizationId)}`,
    { method: "DELETE" },
    204,
    null,
  );
}

function parseScenarioTransactionArray(value: unknown): ScenarioTransaction[] {
  if (!Array.isArray(value)) throw new TypeError("Resposta de parcelas inválida.");
  return value.map(parseScenarioTransaction);
}

export function generateInstallments(
  scenarioId: string,
  write: GenerateInstallmentsWrite,
): Promise<ScenarioTransaction[]> {
  return send(
    `/api/scenarios/${encodeURIComponent(scenarioId)}/generate-installments`,
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    201,
    parseScenarioTransactionArray,
  );
}

export function readjustInstallments(
  scenarioId: string,
  write: ReadjustWrite,
): Promise<ScenarioTransaction[]> {
  return send(
    `/api/scenarios/${encodeURIComponent(scenarioId)}/readjust`,
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(write),
    },
    200,
    parseScenarioTransactionArray,
  );
}
