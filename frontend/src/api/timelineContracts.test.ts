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
  monthly_breakdown: [
    { month: "2026-08-01", income: "2000.00", expense: "100.00", result: "1900.00" },
  ],
  category_breakdown: [
    {
      category_id: "000433b6-3094-5a9c-87df-465b70574a4b",
      category_name: "Supermercado",
      amount: "100.00",
      percentage: "100.00",
    },
    { category_id: null, category_name: "Sem categoria", amount: "0.00", percentage: "0.00" },
  ],
  simulation: null,
  scenario_impacts: [],
  month_over_month: null,
  year_over_year: null,
  category_evolution: null,
};

describe("timeline contracts", () => {
  it("accepts a well-formed response", () => {
    expect(parseTimelineResponse(validResponse)).toEqual(validResponse);
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
    { ...validResponse, monthly_breakdown: "not-an-array" },
    { ...validResponse, category_breakdown: [{ category_id: "not-a-uuid", category_name: "X", amount: "1.00", percentage: "0" }] },
  ])("rejects malformed responses", (payload) => {
    expect(() => parseTimelineResponse(payload)).toThrow();
  });
});
