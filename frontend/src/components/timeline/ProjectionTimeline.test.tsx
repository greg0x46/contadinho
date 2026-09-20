import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { cloneElement, type ReactElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { TimelineEntry, TimelineSeries } from "../../api/contracts";
import { ProjectionTimeline } from "./ProjectionTimeline";

// The tooltip only renders while recharts says a point is hovered, which a
// jsdom chart never reports — so the stub renders its content as if the day
// under `data-hover-date` were hovered.
let hoveredDate: string | null = null;

vi.mock("recharts", () => ({
  CartesianGrid: () => null,
  Legend: () => null,
  Line: ({ dataKey, type }: { dataKey: string; type: string }) => (
    <output data-testid={`line-${dataKey}-type`}>{type}</output>
  ),
  LineChart: ({
    data,
    children,
    onClick,
  }: {
    data: { date: string; balance: number }[];
    children: ReactNode;
    onClick?: (state: { activeLabel?: string }) => void;
  }) => (
    <>
      <output data-testid="chart-data">{JSON.stringify(data)}</output>
      <button type="button" onClick={() => onClick?.({ activeLabel: hoveredDate ?? undefined })}>
        chart surface
      </button>
      {children}
    </>
  ),
  ReferenceDot: () => null,
  ReferenceLine: () => null,
  ResponsiveContainer: ({ children }: { children: ReactNode }) => <>{children}</>,
  Tooltip: ({ content }: { content: ReactElement<Record<string, unknown>> }) =>
    hoveredDate
      ? cloneElement(content, {
          active: true,
          label: hoveredDate,
          payload: [{ name: "Saldo realizado", value: 1000 }],
        })
      : null,
  XAxis: () => null,
  YAxis: () => null,
}));

function entry(overrides: Partial<TimelineEntry> & { description: string; amount: string }): TimelineEntry {
  return {
    date: "2026-08-22",
    reportable_amount: overrides.amount,
    category_id: null,
    category_name: "Sem categoria",
    tier: "realizado",
    source: "real",
    source_ref_id: overrides.description,
    scenario_id: null,
    ...overrides,
  };
}

const series: TimelineSeries = {
  points: [
    { date: "2026-08-10", balance: "900.00", inflow: "0.00", outflow: "0.00", lowest_tier: "realizado" },
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

type ChartRow = { date: string; balance: number; pastBalance?: number; futureBalance?: number };

function chartData(): ChartRow[] {
  return JSON.parse(screen.getByTestId("chart-data").textContent ?? "[]") as ChartRow[];
}

describe("ProjectionTimeline", () => {
  beforeEach(() => {
    hoveredDate = null;
  });

  it("anchors today's plotted balance to the summary's starting balance", () => {
    render(<ProjectionTimeline series={series} referenceDate="2026-08-22" />);

    const data = chartData();
    expect(data[1]).toMatchObject({ date: "2026-08-22", balance: 1000 });
    expect(data[2]).toMatchObject({ date: "2026-08-31", balance: 700 });
    expect(screen.getByTestId("line-pastBalance-type")).toHaveTextContent("stepAfter");
  });

  it("splits the line at the reference date, sharing that day between both halves", () => {
    render(<ProjectionTimeline series={series} referenceDate="2026-08-22" />);

    const data = chartData();
    expect(data[0]).toMatchObject({ date: "2026-08-10", pastBalance: 900 });
    expect(data[0].futureBalance).toBeUndefined();
    expect(data[1]).toMatchObject({ date: "2026-08-22", pastBalance: 1000, futureBalance: 1000 });
    expect(data[2]).toMatchObject({ date: "2026-08-31", futureBalance: 700 });
    expect(data[2].pastBalance).toBeUndefined();
  });

  it("omits the past line when the window starts at the reference date", () => {
    render(<ProjectionTimeline series={series} referenceDate="2026-08-10" />);

    expect(screen.queryByTestId("line-pastBalance-type")).toBeNull();
    expect(screen.getByTestId("line-futureBalance-type")).toBeVisible();
  });

  it("lists the day's entries in the tooltip, largest first, counting the rest", () => {
    hoveredDate = "2026-08-22";
    const withEntries: TimelineSeries = {
      ...series,
      entries: [
        entry({ description: "Café", amount: "-12.00" }),
        entry({ description: "Aluguel", amount: "-1800.00" }),
        entry({ description: "Salário", amount: "5000.00" }),
        entry({ description: "Mercado", amount: "-320.00" }),
        entry({ description: "Farmácia", amount: "-90.00" }),
        entry({ description: "Assinatura de outro dia", amount: "-50.00", date: "2026-08-10" }),
      ],
    };
    render(<ProjectionTimeline series={withEntries} referenceDate="2026-08-22" />);

    expect(screen.getByText("Salário")).toBeVisible();
    expect(screen.getByText("Aluguel")).toBeVisible();
    expect(screen.getByText("Mercado")).toBeVisible();
    expect(screen.getByText("Farmácia")).toBeVisible();
    // Café is the smallest of the five, so it becomes the counted remainder.
    expect(screen.queryByText("Café")).toBeNull();
    expect(screen.getByText("e mais 1")).toBeVisible();
    expect(screen.queryByText("Assinatura de outro dia")).toBeNull();
  });

  it("reports a clicked past day but never a projected one", async () => {
    const user = userEvent.setup();
    const onSelectDay = vi.fn();
    render(<ProjectionTimeline series={series} referenceDate="2026-08-22" onSelectDay={onSelectDay} />);

    hoveredDate = "2026-08-10";
    await user.click(screen.getByRole("button", { name: "chart surface" }));
    expect(onSelectDay).toHaveBeenCalledWith("2026-08-10");

    onSelectDay.mockClear();
    hoveredDate = "2026-08-31";
    await user.click(screen.getByRole("button", { name: "chart surface" }));
    expect(onSelectDay).not.toHaveBeenCalled();
  });
});
