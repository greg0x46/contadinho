import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as timelineApi from "../api/timeline";
import type { TimelineResponse } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { FinancialReportPage } from "./FinancialReportPage";

vi.mock("../api/timeline");

const emptyResponse: TimelineResponse = {
  base: {
    points: [],
    entries: [],
    starting_balance: "0.00",
    lowest_balance: { date: "2026-08-15", balance: "0.00", inflow: "0.00", outflow: "0.00", lowest_tier: "realizado" },
    first_negative: null,
  },
  period_totals: { income: "0.00", expense: "0.00", result: "0.00" },
  monthly_breakdown: [],
  category_breakdown: [],
  simulation: null,
  scenario_impacts: [],
  month_over_month: null,
  year_over_year: null,
  category_evolution: null,
};

const populatedResponse: TimelineResponse = {
  base: {
    points: [
      { date: "2026-08-15", balance: "1900.00", inflow: "2000.00", outflow: "100.00", lowest_tier: "realizado" },
    ],
    entries: [
      {
        date: "2026-08-01",
        description: "Mercado",
        amount: "-100.00",
        reportable_amount: "-100.00",
        investment_transfer_amount: "0",
        investment_transfer_kind: null,
        category_id: "000433b6-3094-5a9c-87df-465b70574a4b",
        category_name: "Supermercado",
        tier: "realizado",
        source: "real",
        source_ref_id: "11111111-1111-4111-8111-111111111111",
        scenario_id: null,
      },
    ],
    starting_balance: "1900.00",
    lowest_balance: { date: "2026-08-15", balance: "1900.00", inflow: "0.00", outflow: "0.00", lowest_tier: "realizado" },
    first_negative: null,
  },
  period_totals: { income: "2000.00", expense: "100.00", result: "1900.00" },
  monthly_breakdown: [
    {
      month: "2026-08-01",
      income: "2000.00",
      expense: "100.00",
      result: "1900.00",
      investment_contributions: "0.00",
      investment_withdrawals: "0.00",
    },
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

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <FinancialReportPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("FinancialReportPage", () => {
  beforeEach(() => {
    vi.setSystemTime(new Date("2026-08-15T12:00:00Z"));
  });

  it("shows a distinct 'sem movimentações' state instead of a zeroed card", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValue(emptyResponse);
    renderPage();
    expect(await screen.findByText("Sem movimentações no período.")).toBeVisible();
  });

  it("renders selected-month and accumulated summaries as separate groups", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValue(populatedResponse);
    renderPage();
    expect(await screen.findByText("Mês selecionado")).toBeVisible();
    expect(screen.getByText("Acumulado no ano")).toBeVisible();
  });

  it("renders the category impact list with a clickable row", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValue(populatedResponse);
    renderPage();
    expect(await screen.findByText("Supermercado")).toBeVisible();
    expect(screen.getByText("Sem categoria")).toBeVisible();
  });

  // The projection assertions that used to live here — current/projected/
  // lowest balance, the negative-balance warning, the scenario multi-select
  // and the base×simulation compare — moved out with the projection itself
  // when the dashboard took it over. What survived is covered by
  // HomePage.test.tsx ("Saldo hoje", the horizon options, the retry state);
  // the simulation compare and per-scenario impact are not rendered anywhere
  // today, so there is nothing left here to assert about them.
  it("shows month-over-month and year-over-year comparisons when present, omitting when absent", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValue({
      ...populatedResponse,
      month_over_month: { current: "1900.00", previous: "1000.00", delta_percent: "90" },
    });
    renderPage();
    expect(await screen.findByText("Resultado vs. mês anterior")).toBeVisible();
    expect(screen.queryByText("Acumulado vs. mesmo período ano anterior")).not.toBeInTheDocument();
  });

  it("keeps aportes and resgates out of the page when nothing was invested", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValue(populatedResponse);
    renderPage();
    expect((await screen.findAllByText("Receitas")).length).toBeGreaterThan(0);
    expect(screen.queryByText("Movimentações de investimento")).not.toBeInTheDocument();
  });

  it("reads aportes and resgates apart from receitas and despesas", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValue({
      ...populatedResponse,
      monthly_breakdown: [
        {
          ...populatedResponse.monthly_breakdown[0]!,
          investment_contributions: "500.00",
          investment_withdrawals: "80.00",
        },
      ],
    });
    renderPage();
    expect(await screen.findByText("Movimentações de investimento")).toBeVisible();
    expect(screen.getByText("Aportes no mês")).toBeVisible();
    expect(screen.getByText("Resgates no mês")).toBeVisible();
    expect(screen.getAllByText("R$ 500,00").length).toBeGreaterThan(0);
    expect(screen.getAllByText("R$ 80,00").length).toBeGreaterThan(0);
    // The aporte must not be folded into despesas: the month keeps its own
    // expense reading untouched.
    expect(screen.getAllByText("R$ 100,00").length).toBeGreaterThan(0);
  });

  it("opens the category evolution chart from the 'ver evolução' action", async () => {
    const user = userEvent.setup();
    vi.mocked(timelineApi.getTimeline).mockImplementation((params) =>
      Promise.resolve({
        ...populatedResponse,
        category_evolution: params.categoryEvolutionId
          ? [{ month: "2026-08-01", amount: "100.00" }]
          : null,
      }),
    );
    renderPage();
    const buttons = await screen.findAllByText("Ver evolução");
    await user.click(buttons[0]);
    expect(await screen.findByText("Evolução de Supermercado")).toBeVisible();
  });
});
