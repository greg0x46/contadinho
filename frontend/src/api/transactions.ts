import {
  parseProblem,
  parseSpendingByCategory,
  parseTransactionCategoryResult,
  parseTransactionInclusionResult,
  parseTransactionQueryResult,
  type Problem,
  type SpendingByCategory,
  type TransactionCategoryResult,
  type TransactionInclusionResult,
  type TransactionInclusionState,
  type TransactionQuery,
  type TransactionQueryResult,
} from "./contracts";
import { ApiError, isAbortError } from "./problems";

export async function queryTransactions(
  query: TransactionQuery,
  signal?: AbortSignal,
): Promise<TransactionQueryResult> {
  let response: Response;
  try {
    response = await fetch("/api/transactions/query", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(query),
      signal,
    });
  } catch (error) {
    if (isAbortError(error)) throw error;
    throw new ApiError("transport", "Não foi possível consultar as transações.");
  }

  const contentType = response.headers.get("content-type")?.split(";")[0].trim();
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new ApiError("response", "A resposta de transações não pôde ser confirmada.");
  }

  if (!response.ok) {
    let problem: Problem | undefined;
    if (contentType === "application/problem+json") {
      try {
        problem = parseProblem(body);
      } catch {
        problem = undefined;
      }
    }
    throw new ApiError(
      "response",
      problem?.detail ?? problem?.title ?? "Não foi possível consultar as transações.",
      problem,
    );
  }
  if (contentType !== "application/json") {
    throw new ApiError("response", "A resposta de transações não pôde ser confirmada.");
  }
  try {
    return parseTransactionQueryResult(body);
  } catch {
    throw new ApiError("response", "A resposta de transações não pôde ser confirmada.");
  }
}

export async function getSpendingByCategory(
  timezone: string,
  signal?: AbortSignal,
  scenarioId?: string,
): Promise<SpendingByCategory> {
  const params = new URLSearchParams({ timezone });
  if (scenarioId !== undefined) params.set("scenario_id", scenarioId);
  let response: Response;
  try {
    response = await fetch(`/api/transactions/spending-by-category?${params.toString()}`, {
      method: "GET",
      headers: { Accept: "application/json" },
      signal,
    });
  } catch (error) {
    if (isAbortError(error)) throw error;
    throw new ApiError("transport", "Não foi possível consultar os gastos por categoria.");
  }

  const contentType = response.headers.get("content-type")?.split(";")[0].trim();
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new ApiError("response", "A resposta de gastos por categoria não pôde ser confirmada.");
  }

  if (!response.ok) {
    let problem: Problem | undefined;
    if (contentType === "application/problem+json") {
      try {
        problem = parseProblem(body);
      } catch {
        problem = undefined;
      }
    }
    throw new ApiError(
      "response",
      problem?.detail ?? problem?.title ?? "Não foi possível consultar os gastos por categoria.",
      problem,
    );
  }
  if (contentType !== "application/json") {
    throw new ApiError("response", "A resposta de gastos por categoria não pôde ser confirmada.");
  }
  try {
    return parseSpendingByCategory(body);
  } catch {
    throw new ApiError("response", "A resposta de gastos por categoria não pôde ser confirmada.");
  }
}

export async function setTransactionInclusion(
  transactionId: string,
  state: TransactionInclusionState,
): Promise<TransactionInclusionResult> {
  let response: Response;
  try {
    response = await fetch(`/api/transactions/${transactionId}/inclusion`, {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ state }),
    });
  } catch (error) {
    if (isAbortError(error)) throw error;
    throw new ApiError("transport", "Não foi possível salvar a decisão da transação.");
  }

  const contentType = response.headers.get("content-type")?.split(";")[0].trim();
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new ApiError("response", "A confirmação da decisão é inválida.");
  }
  if (!response.ok) {
    let problem: Problem | undefined;
    if (contentType === "application/problem+json") {
      try {
        problem = parseProblem(body);
      } catch {
        problem = undefined;
      }
    }
    throw new ApiError(
      "response",
      problem?.detail ?? problem?.title ?? "Não foi possível salvar a decisão da transação.",
      problem,
    );
  }
  if (contentType !== "application/json") {
    throw new ApiError("response", "A confirmação da decisão é inválida.");
  }
  try {
    return parseTransactionInclusionResult(body);
  } catch {
    throw new ApiError("response", "A confirmação da decisão é inválida.");
  }
}

export async function setTransactionCategory(
  transactionId: string,
  categoryId: string,
): Promise<TransactionCategoryResult> {
  let response: Response;
  try {
    response = await fetch(`/api/transactions/${transactionId}/category`, {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ category_id: categoryId }),
    });
  } catch (error) {
    if (isAbortError(error)) throw error;
    throw new ApiError("transport", "Não foi possível salvar a categoria da transação.");
  }

  const contentType = response.headers.get("content-type")?.split(";")[0].trim();
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new ApiError("response", "A confirmação da categoria é inválida.");
  }
  if (!response.ok) {
    let problem: Problem | undefined;
    if (contentType === "application/problem+json") {
      try {
        problem = parseProblem(body);
      } catch {
        problem = undefined;
      }
    }
    throw new ApiError(
      "response",
      problem?.detail ?? problem?.title ?? "Não foi possível salvar a categoria da transação.",
      problem,
    );
  }
  if (contentType !== "application/json") {
    throw new ApiError("response", "A confirmação da categoria é inválida.");
  }
  try {
    return parseTransactionCategoryResult(body);
  } catch {
    throw new ApiError("response", "A confirmação da categoria é inválida.");
  }
}
