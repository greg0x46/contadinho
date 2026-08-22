import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import type { TimelineSeries } from "../../api/contracts";
import { ProjectionTimeline } from "./ProjectionTimeline";

vi.mock("recharts", () => ({
  CartesianGrid: () => null,
  Legend: () => null,
  Line: ({ dataKey, type }: { dataKey: string; type: string }) => (
    <output data-testid={`line-${dataKey}-type`}>{type}</output>
  ),
  LineChart: ({ data, children }: { data: { date: string; balance: number }[]; children: ReactNode }) => (
    <>
      <output data-testid="chart-data">{JSON.stringify(data)}</output>
      {children}
    </>
  ),
  ReferenceDot: () => null,
  ReferenceLine: () => null,
  ResponsiveContainer: ({ children }: { children: ReactNode }) => <>{children}</>,
  Tooltip: () => null,
  XAxis: () => null,
  YAxis: () => null,
}));

const series: TimelineSeries = {
  points: [
    { date: "2026-08-22", balance: "800.00", inflow: "0.00", outflow: "0.00", lowest_tier: "realizado" },
    { date: "2026-08-31", balance: "700.00", inflow: "0.00", outflow: "100.00", lowest_tier: "projetado" },
  ],
  entries: [],
  starting_balance: "1000.00",
  lowest_balance: {
    date: "2026-08-31",
    balance: "700.00",
    inflow: "0.00",
    outflow: "100.00",
    lowest_tier: "projetado",
  },
  first_negative: null,
};

describe("ProjectionTimeline", () => {
  it("anchors today's plotted balance to the summary's starting balance", () => {
    render(<ProjectionTimeline series={series} referenceDate="2026-08-22" />);

    const data = JSON.parse(screen.getByTestId("chart-data").textContent ?? "[]") as {
      date: string;
      balance: number;
    }[];
    expect(data[0]).toMatchObject({ date: "2026-08-22", balance: 1000 });
    expect(data[1]).toMatchObject({ date: "2026-08-31", balance: 700 });
    expect(screen.getByTestId("line-balance-type")).toHaveTextContent("stepAfter");
  });
});
