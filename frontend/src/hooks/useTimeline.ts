import { useQuery } from "@tanstack/react-query";

import { getTimeline } from "../api/timeline";
import type { TimelineParams } from "../api/contracts";

export function useTimeline(params: TimelineParams) {
  const query = useQuery({
    queryKey: ["timeline", params],
    queryFn: ({ signal }) => getTimeline(params, signal),
  });

  return {
    base: query.data?.base ?? null,
    simulation: query.data?.simulation ?? null,
    scenarioImpacts: query.data?.scenario_impacts ?? [],
    monthlyBreakdown: query.data?.monthly_breakdown ?? [],
    categoryBreakdown: query.data?.category_breakdown ?? [],
    monthOverMonth: query.data?.month_over_month ?? null,
    yearOverYear: query.data?.year_over_year ?? null,
    categoryEvolution: query.data?.category_evolution ?? null,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}
