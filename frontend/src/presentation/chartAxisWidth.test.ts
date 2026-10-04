import { describe, expect, it } from "vitest";

import { yAxisWidth } from "./chartAxisWidth";

describe("yAxisWidth", () => {
  it("is wide enough for the longest tick, including the rounded ones", () => {
    // "-1,5 mi" is the longest label; "-500 mil" (8 characters) must also fit.
    expect(yAxisWidth([-1_500_000, 120_000])).toBeGreaterThanOrEqual(8 * 7 + 8);
  });

  it("stays narrow when the labels are short", () => {
    expect(yAxisWidth([0, 800])).toBeLessThan(50);
  });
});
