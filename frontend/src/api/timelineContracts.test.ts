import { describe, expect, it } from "vitest";

import { parseTimelineResponse } from "./contracts";

const validResponse = {
  base: {
    points: [
      { date: "2026-08-01", balance: "700.00", inflow: "0.00", outflow: "100.00", lowest_tier: "realizado" },
      { date: "2026-08-15", balance: "1000.00", inflow: "2000.00", outflow: "0.00", lowest_tier: "realizado" },
    ],
    entries: [
      {
        date: "2026-08-01",
        description: "Mercado",
        amount: "-100.00",
        category_id: "000433b6-3094-5a9c-87df-465b70574a4b",
        category_name: "Supermercado",
        tier: "realizado",
        source: "real",
        source_ref_id: "11111111-1111-4111-8111-111111111111",
        scenario_id: null,
      },
    ],
    starting_balance: "1000.00",
    lowest_balance: { date: "2026-08-15", balance: "1000.00", inflow: "0.00", outflow: "0.00", lowest_tier: "realizado" },
    first_negative: null,
  },
  period_totals: { income: "2000.00", expense: "100.00", result: "1900.00" },
  simulation: null,
  scenario_impacts: [],
};

describe("timeline contracts", () => {
  it("accepts a well-formed response", () => {
    // This fixture predates the investment reading, like a response from an
    // older server: the absent reportable_amount is filled from amount
    // instead of rejecting the payload.
    expect(parseTimelineResponse(validResponse)).toEqual({
      ...validResponse,
      base: {
        ...validResponse.base,
        entries: validResponse.base.entries.map((entry) => ({
          ...entry,
          reportable_amount: entry.amount,
        })),
      },
    });
  });

  it("accepts a null first_negative and scenario_id", () => {
    expect(() => parseTimelineResponse(validResponse)).not.toThrow();
  });

  it.each([
    { ...validResponse, base: { ...validResponse.base, starting_balance: "not-a-number" } },
    { ...validResponse, base: { ...validResponse.base, points: "not-an-array" } },
    {
      ...validResponse,
      base: {
        ...validResponse.base,
        entries: [{ ...validResponse.base.entries[0], tier: "unknown" }],
      },
    },
    { ...validResponse, scenario_impacts: "not-an-array" },
    { ...validResponse, period_totals: { income: "1.00", expense: "muito", result: "0" } },
  ])("rejects malformed responses", (payload) => {
    expect(() => parseTimelineResponse(payload)).toThrow();
  });
});
