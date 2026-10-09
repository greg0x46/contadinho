import type { Dayjs } from "dayjs";

import { divideBRLFloor } from "./money";

/** Days left in today's month, today included: 1 on the last day. */
export function daysRemainingInMonth(today: Dayjs): number {
  return today.endOf("month").startOf("day").diff(today.startOf("day"), "day") + 1;
}

/**
 * How much can be spent per day until month-end without the projected balance
 * dipping below zero: the lowest projected balance from today on, split over
 * the days left. A negative or zero low point leaves nothing to spend.
 */
export function dailyAllowance(lowestBalance: string, today: Dayjs): string {
  const available = lowestBalance.trim().startsWith("-") ? "0.00" : lowestBalance;
  return divideBRLFloor(available, daysRemainingInMonth(today));
}
