import {
  parseProblem,
  parseTimelineResponse,
  type Problem,
  type TimelineParams,
  type TimelineResponse,
} from "./contracts";
import { ApiError, isAbortError } from "./problems";

const defaultMessage = "Não foi possível carregar o relatório financeiro.";

function buildQuery(params: TimelineParams): string {
  const search = new URLSearchParams({
    reference_date: params.referenceDate,
    from: params.from,
    to: params.to,
  });
  if (params.accountIds && params.accountIds.length > 0) search.set("account_ids", params.accountIds.join(","));
  if (params.categoryIds && params.categoryIds.length > 0) search.set("category_ids", params.categoryIds.join(","));
  if (params.cardNumbers && params.cardNumbers.length > 0) search.set("card_numbers", params.cardNumbers.join(","));
  if (params.scenarioIds && params.scenarioIds.length > 0) search.set("scenario_ids", params.scenarioIds.join(","));
  if (params.yearOverYear) search.set("year_over_year", "true");
  if (params.categoryEvolutionId) search.set("category_evolution_id", params.categoryEvolutionId);
  return search.toString();
}

export async function getTimeline(params: TimelineParams, signal?: AbortSignal): Promise<TimelineResponse> {
  let response: Response;
  try {
    response = await fetch(`/api/timeline?${buildQuery(params)}`, {
      method: "GET",
      headers: { Accept: "application/json" },
      signal,
    });
  } catch (error) {
    if (isAbortError(error)) throw error;
    throw new ApiError("transport", defaultMessage);
  }

  const contentType = response.headers.get("content-type")?.split(";")[0].trim();

  if (response.status !== 200) {
    let problem: Problem | undefined;
    if (contentType === "application/problem+json") {
      try {
        problem = parseProblem(await response.json());
      } catch {
        problem = undefined;
      }
    }
    throw new ApiError("response", problem?.detail ?? problem?.title ?? defaultMessage, problem);
  }

  if (contentType !== "application/json") {
    throw new ApiError("response", defaultMessage);
  }
  try {
    return parseTimelineResponse(await response.json());
  } catch {
    throw new ApiError("response", defaultMessage);
  }
}
