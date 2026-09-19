import { useMemo } from "react";

import type { HomePeriod } from "./useHomePeriod";
import { useTimelineDataRange } from "./useTimelineDataRange";

/**
 * Resolves a HomePeriod into concrete dates. "Todo o período" carries no
 * dates of its own, so its bounds come from the database instead — the
 * oldest transaction, the last planned installment.
 */
export function useHomePeriodBounds(period: HomePeriod) {
  const wholePeriod = period.from === null || period.to === null;
  const dataRange = useTimelineDataRange(wholePeriod);
  const bounds = useMemo(() => {
    if (!wholePeriod) return { from: period.from as string, to: period.to as string };
    return dataRange.range ? { from: dataRange.range.from, to: dataRange.range.to } : null;
  }, [wholePeriod, period.from, period.to, dataRange.range]);

  return {
    bounds,
    wholePeriod,
    isLoading: dataRange.isLoading,
    error: dataRange.error,
    refetch: dataRange.refetch,
  };
}
