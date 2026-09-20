import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import dayjs from "dayjs";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as timelineApi from "../api/timeline";
import * as transactionsApi from "../api/transactions";
import type {
  CategoryBreakdown,
  TimelineResponse,
} from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { HomePage } from "./HomePage";

vi.mock("../api/timeline");
vi.mock("../api/transactions");

const spendingByCategory: CategoryBreakdown = {
  classification: "outflow",
  month: "2026-08",
  currency_code: "BRL",
  total: "150.00",
  items: [
    {
      category_id: "cat-mercado",
      category_name: "Mercado",
      category_icon: "shopping-cart",
      category_color: "#2a78d6",
      amount: "100.00",
      source: "real",
    },
    {
      category_id: null,
      category_name: "Sem categoria",
      category_icon: "",
      category_color: "",
      amount: "50.00",
      source: "real",
    },
  ],
};

const projection: TimelineResponse = {
  base: {
    points: [
      { date: "2026-08-15", balance: "1900.00", inflow: "620.00", outflow: "0.00", lowest_tier: "realizado" },
      { date: "2026-11-30", balance: "1500.00", inflow: "0.00", outflow: "-950.00", lowest_tier: "projetado" },
    ],
    entries: [],
    starting_balance: "1900.00",
    lowest_balance: {
      date: "2026-11-30",
      balance: "1500.00",
      inflow: "0.00",
      outflow: "-950.00",
      lowest_tier: "projetado",
    },
    first_negative: null,
  },
  period_totals: { income: "620.00", expense: "950.00", result: "-330.00" },
  simulation: null,
  scenario_impacts: [],
};

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <HomePage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  // The projection window is remembered per browser, so one test's choice
  // would otherwise decide the next test's request.
  window.localStorage.clear();
  vi.mocked(transactionsApi.getCategoryBreakdown).mockResolvedValue(spendingByCategory);
  vi.mocked(timelineApi.getTimeline).mockResolvedValue(projection);
  vi.mocked(timelineApi.getTimelineDataRange).mockResolvedValue({ from: "2024-03-07", to: "2028-06-15" });
});

describe("PeriodBalanceCard", () => {
  it("shows entradas, saídas and the resulting saldo for the period", async () => {
    renderPage();
    expect(await screen.findByText(/-R\$\s330,00/)).toBeVisible();
    expect(screen.getByRole("link", { name: /Entradas.*620,00/ })).toBeVisible();
    expect(screen.getByRole("link", { name: /Saídas.*950,00/ })).toBeVisible();
    expect(screen.getByRole("img", { name: /Entradas:.*620,00.*Saídas:.*950,00/ })).toBeVisible();
  });

  it("links entradas and saídas to the matching transactions for the period", async () => {
    renderPage();
    await screen.findByText(/-R\$\s330,00/);
    const today = dayjs();
    expect(screen.getByRole("link", { name: /Entradas/ })).toHaveAttribute(
      "href",
      `/transacoes?date_from=${today.startOf("month").format("YYYY-MM-DD")}&date_to=${today.endOf("month").format("YYYY-MM-DD")}&classification=inflow`,
    );
    expect(screen.getByRole("link", { name: /Saídas/ })).toHaveAttribute(
      "href",
      `/transacoes?date_from=${today.startOf("month").format("YYYY-MM-DD")}&date_to=${today.endOf("month").format("YYYY-MM-DD")}&classification=outflow`,
    );
  });

  it("shows a plus sign only for a positive saldo", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValue({
      ...projection,
      period_totals: { income: "400.00", expense: "100.00", result: "300.00" },
    });
    renderPage();
    expect(await screen.findByText(/\+R\$\s300,00/)).toBeVisible();
  });

  it("shows an empty state instead of a zero-width proportion bar when there is no movement", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValue({
      ...projection,
      period_totals: { income: "0.00", expense: "0.00", result: "0.00" },
    });
    renderPage();
    expect(await screen.findByText("Nenhuma movimentação no período")).toBeVisible();
    expect(screen.queryByRole("img", { name: /Entradas:/ })).not.toBeInTheDocument();
  });

  it("shows a loading state before the period's timeline resolves", async () => {
    let resolve!: () => void;
    vi.mocked(timelineApi.getTimeline).mockReturnValue(new Promise((done) => { resolve = () => done(projection); }));
    renderPage();
    expect(screen.getByText("Carregando o saldo do período…")).toBeVisible();
    expect(document.querySelector(".period-balance-figure")).toBeNull();
    resolve();
    expect(await screen.findByText(/-R\$\s330,00/)).toBeVisible();
  });

  it("retries after a failure", async () => {
    const user = userEvent.setup();
    // Both period widgets share the same failing timeline query, so each
    // shows its own retry button — this one must click the right card's.
    vi.mocked(timelineApi.getTimeline).mockRejectedValueOnce(new Error("boom"));
    renderPage();
    const message = await screen.findByText("Não foi possível carregar o saldo do período.");
    expect(document.querySelector(".period-balance-figure")).toBeNull();
    const calls = vi.mocked(timelineApi.getTimeline).mock.calls.length;
    await user.click(within(message.closest(".ant-alert")!).getByRole("button", { name: "Tentar novamente" }));
    expect(await screen.findByText(/-R\$\s330,00/)).toBeVisible();
    expect(timelineApi.getTimeline).toHaveBeenCalledTimes(calls + 1);
  });

  it("uses period_totals for the whole period, active scenarios included", async () => {
    const user = userEvent.setup();
    vi.mocked(timelineApi.getTimeline).mockResolvedValue({
      ...projection,
      period_totals: { income: "700.00", expense: "0.00", result: "700.00" },
    });
    renderPage();
    await screen.findByText("Saldo hoje");
    await user.click(screen.getByRole("button", { name: "Selecionar período" }));
    await user.click(await screen.findByRole("button", { name: "Todo o período" }));
    expect(await screen.findByText(/\+R\$\s700,00/)).toBeVisible();
  });
});

function timelineParams() {
  return vi.mocked(timelineApi.getTimeline).mock.calls.map((call) => call[0]);
}

describe("HomePage", () => {
  it("renders a compact projection summary on the Home dashboard", async () => {
    renderPage();
    expect(await screen.findByText("Saldo hoje")).toBeVisible();
    expect(screen.getByText("Evolução do saldo")).toBeVisible();

    const today = dayjs();
    expect(screen.getByRole("button", { name: "Selecionar período" })).toHaveTextContent(String(today.year()));
  });

  it("asks for the current month by default, anchored on today", async () => {
    renderPage();
    await screen.findByText("Saldo hoje");

    const today = dayjs();
    expect(timelineParams()).toContainEqual({
      // The anchor stays today even though the window opens earlier:
      // days before it are the real balance, days after it the projection.
      referenceDate: today.format("YYYY-MM-DD"),
      from: today.startOf("month").format("YYYY-MM-DD"),
      to: today.endOf("month").format("YYYY-MM-DD"),
    });
  });

  it("offers the same period shortcuts as the transactions filter", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Saldo hoje");

    await user.click(screen.getByRole("button", { name: "Selecionar período" }));

    const presets = await screen.findByText("Este mês");
    expect(
      [...presets.closest(".filter-period-presets")!.children].map((button) => button.textContent),
    ).toEqual(["Este mês", "Mês passado", "Últimos 30 dias", "Este ano", "Todo o período"]);
  });

  it("walks back a year at a time with the period arrows", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Saldo hoje");

    await user.click(screen.getByRole("button", { name: "Selecionar período" }));
    await user.click(await screen.findByRole("button", { name: "Este ano" }));
    await user.click(screen.getByRole("button", { name: "Ano anterior" }));

    const today = dayjs();
    const previous = today.subtract(1, "year");
    await vi.waitFor(() =>
      expect(timelineParams()).toContainEqual({
        // The anchor stays today even when the window is entirely in the past.
        referenceDate: today.format("YYYY-MM-DD"),
        from: previous.startOf("year").format("YYYY-MM-DD"),
        to: previous.endOf("year").format("YYYY-MM-DD"),
      }),
    );
    expect(await screen.findByText(String(previous.year()))).toBeVisible();
  });

  it("applies a custom range to every widget and their links", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Mercado");
    expect(screen.getAllByRole("button", { name: "Selecionar período" })).toHaveLength(1);
    await user.click(screen.getByRole("button", { name: "Selecionar período" }));
    await user.clear(screen.getByLabelText("Data inicial"));
    await user.type(screen.getByLabelText("Data inicial"), "2025-02-15");
    await user.clear(screen.getByLabelText("Data final"));
    await user.type(screen.getByLabelText("Data final"), "2025-03-20");
    await user.click(screen.getByRole("button", { name: "Confirmar período" }));
    await screen.findByText("Mercado");
    expect(transactionsApi.getCategoryBreakdown).toHaveBeenLastCalledWith(expect.any(String), { from: "2025-02-15", to: "2025-03-20" }, "outflow", expect.any(AbortSignal));
    expect(timelineParams()).toContainEqual(expect.objectContaining({ from: "2025-02-15", to: "2025-03-20" }));
    const params = new URL(screen.getByRole("link", { name: /Mercado/ }).getAttribute("href")!, "http://localhost").searchParams;
    expect(params.get("date_from")).toBe("2025-02-15");
    expect(params.get("date_to")).toBe("2025-03-20");
    expect(screen.getByRole("link", { name: /Entradas/ })).toHaveAttribute(
      "href",
      "/transacoes?date_from=2025-02-15&date_to=2025-03-20&classification=inflow",
    );
  });

  it("allows a future month and cancels the previous category request", async () => {
    vi.mocked(transactionsApi.getCategoryBreakdown).mockImplementationOnce(() => new Promise(() => {}));
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Saldo hoje");
    const previousSignal = vi.mocked(transactionsApi.getCategoryBreakdown).mock.calls[0][3]!;
    await user.click(screen.getByRole("button", { name: "Próximo mês" }));
    await screen.findByText("Mercado");
    const next = dayjs().add(1, "month");
    const period = { from: next.startOf("month").format("YYYY-MM-DD"), to: next.endOf("month").format("YYYY-MM-DD") };
    expect(transactionsApi.getCategoryBreakdown).toHaveBeenLastCalledWith(expect.any(String), period, "outflow", expect.any(AbortSignal));
    expect(timelineParams()).toContainEqual(expect.objectContaining(period));
    expect(previousSignal.aborted).toBe(true);
  });

  it("ignores the old balance-only preference", async () => {
    window.localStorage.setItem("contadinho.home.saldo-periodo", JSON.stringify({ preset: "this-year" }));
    renderPage();
    await screen.findByText("Saldo hoje");
    expect(timelineParams()).toContainEqual(expect.objectContaining({
      from: dayjs().startOf("month").format("YYYY-MM-DD"),
      to: dayjs().endOf("month").format("YYYY-MM-DD"),
    }));
  });

  it("remains usable when browser storage is unavailable", async () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("blocked"); });
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Mercado");
    await user.click(screen.getByRole("button", { name: "Mês anterior" }));
    await screen.findByText("Mercado");
    const previous = dayjs().subtract(1, "month");
    expect(timelineParams()).toContainEqual(expect.objectContaining({
      from: previous.startOf("month").format("YYYY-MM-DD"),
      to: previous.endOf("month").format("YYYY-MM-DD"),
    }));
  });

  it("reports the period's own low point once the window ends today", async () => {
    const today = dayjs();
    // A window ending today has no point after the reference date, which is
    // what flips the card from "previsto" to "no período".
    const last30Days: TimelineResponse = {
      ...projection,
      base: {
        ...projection.base,
        points: [
          { date: today.subtract(29, "day").format("YYYY-MM-DD"), balance: "300.00", inflow: "0.00", outflow: "0.00", lowest_tier: "realizado" },
          { date: today.format("YYYY-MM-DD"), balance: "1900.00", inflow: "0.00", outflow: "0.00", lowest_tier: "realizado" },
        ],
      },
    };
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Saldo hoje");

    vi.mocked(timelineApi.getTimeline).mockResolvedValue(last30Days);
    await user.click(screen.getByRole("button", { name: "Selecionar período" }));
    await user.click(await screen.findByRole("button", { name: "Últimos 30 dias" }));

    await vi.waitFor(() =>
      expect(timelineParams()).toContainEqual({
        referenceDate: today.format("YYYY-MM-DD"),
        from: today.subtract(29, "day").format("YYYY-MM-DD"),
        to: today.format("YYYY-MM-DD"),
      }),
    );
    // No future left in the window, so the card stops calling it a forecast
    // and reports the lowest balance the period actually saw.
    expect(await screen.findByText(/^Menor saldo no período em/)).toBeVisible();
    expect(screen.getByText(/R\$\s300,00/)).toBeVisible();
  });

  it("asks the API for the bounds of the whole period", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Saldo hoje");

    await user.click(screen.getByRole("button", { name: "Selecionar período" }));
    await user.click(await screen.findByRole("button", { name: "Todo o período" }));

    const today = dayjs();
    await vi.waitFor(() =>
      expect(timelineParams()).toContainEqual({
        referenceDate: today.format("YYYY-MM-DD"),
        from: "2024-03-07",
        to: "2028-06-15",
      }),
    );
    // The header keeps saying "todo o período"; the card spells out the span
    // the database actually answered with.
    expect(await screen.findByText("07/03/2024 – 15/06/2028")).toBeVisible();
    expect(transactionsApi.getCategoryBreakdown).toHaveBeenLastCalledWith(expect.any(String), { from: null, to: null }, "outflow", expect.any(AbortSignal));
    expect(screen.getByRole("link", { name: "Ver transações" })).toHaveAttribute("href", "/transacoes?period=all&classification=outflow");
    expect(screen.getByRole("link", { name: /Mercado/ }).getAttribute("href")).toContain("period=all");
  });

  it("remembers the chosen period across visits", async () => {
    const user = userEvent.setup();
    const first = renderPage();
    await screen.findByText("Saldo hoje");
    await user.click(screen.getByRole("button", { name: "Selecionar período" }));
    await user.click(await screen.findByRole("button", { name: "Este ano" }));
    first.unmount();

    vi.mocked(timelineApi.getTimeline).mockClear();
    renderPage();
    await screen.findByText("Saldo hoje");

    const today = dayjs();
    expect(timelineParams()).toContainEqual({
      referenceDate: today.format("YYYY-MM-DD"),
      from: today.startOf("year").format("YYYY-MM-DD"),
      to: today.endOf("year").format("YYYY-MM-DD"),
    });
  });

  it("resolves a remembered shortcut again instead of freezing its dates", async () => {
    // Stored while 2024 was current; the window must still be *this* year.
    window.localStorage.setItem("contadinho.home.periodo", JSON.stringify({ preset: "this-year" }));
    renderPage();
    await screen.findByText("Saldo hoje");

    const today = dayjs();
    expect(timelineParams()).toContainEqual({
      referenceDate: today.format("YYYY-MM-DD"),
      from: today.startOf("year").format("YYYY-MM-DD"),
      to: today.endOf("year").format("YYYY-MM-DD"),
    });
  });

  it.each([JSON.stringify({ from: "2026-02-30", to: "2026-03-20" }), "decada", JSON.stringify({ from: "2026-12-31", to: "2026-01-01" }), "{"])(
    "falls back to the default window when the stored period is %s",
    async (stored) => {
      window.localStorage.setItem("contadinho.home.periodo", stored);
      renderPage();
      await screen.findByText("Saldo hoje");

      const today = dayjs();
      expect(timelineParams()).toContainEqual({
        referenceDate: today.format("YYYY-MM-DD"),
        from: today.startOf("month").format("YYYY-MM-DD"),
        to: today.endOf("month").format("YYYY-MM-DD"),
      });
    },
  );

  it("keeps current cards on the left and projection metrics above its chart", async () => {
    renderPage();
    await screen.findByText("Saldo hoje");

    const layout = document.querySelector(".dashboard-layout");
    expect(layout?.firstElementChild).toHaveClass("dashboard-current-summary");
    expect(layout?.lastElementChild).toHaveClass("dashboard-widget");

    const summary = document.querySelector(".projection-summary");
    expect(summary?.firstElementChild).toHaveClass("projection-summary-intro");
    expect(summary?.lastElementChild).toHaveClass("timeline-chart");
  });

  it("shows a retry option when the projection fails to load", async () => {
    vi.mocked(timelineApi.getTimeline).mockRejectedValue(new Error("boom"));
    renderPage();
    expect(await screen.findByText("Não foi possível carregar o saldo.")).toBeVisible();
  });


});

describe("SpendingByCategoryCard", () => {
  it("renders each category's share of the month's spending once loaded", async () => {
    renderPage();
    expect(await screen.findByText("Mercado")).toBeVisible();
    expect(screen.getByText("Sem categoria")).toBeVisible();
    expect(screen.getByText(/150,00/)).toBeVisible();
    expect(screen.getByText(/100,00/)).toBeVisible();
    expect(screen.getByText("66,7%")).toBeVisible();
    expect(screen.getByText("33,3%")).toBeVisible();
  });

  it("shows all categories without aggregating smaller shares", async () => {
    vi.mocked(transactionsApi.getCategoryBreakdown).mockResolvedValue({
      month: "2026-08",
      classification: "outflow",
      currency_code: "BRL",
      total: "600.00",
      items: [
        { category_id: "1", category_name: "Um", category_icon: "ellipsis", category_color: "#2a78d6", amount: "100.00", source: "real" },
        { category_id: "2", category_name: "Dois", category_icon: "ellipsis", category_color: "#eb6834", amount: "100.00", source: "real" },
        { category_id: "3", category_name: "Tres", category_icon: "ellipsis", category_color: "#17a2b8", amount: "100.00", source: "real" },
        { category_id: "4", category_name: "Quatro", category_icon: "ellipsis", category_color: "#e64980", amount: "100.00", source: "real" },
        { category_id: "5", category_name: "Cinco", category_icon: "ellipsis", category_color: "#d64545", amount: "100.00", source: "real" },
        { category_id: "6", category_name: "Seis", category_icon: "ellipsis", category_color: "#b8860b", amount: "100.00", source: "real" },
      ],
    });
    renderPage();
    expect(await screen.findByText("Cinco")).toBeVisible();
    expect(screen.queryByText("Outras categorias")).not.toBeInTheDocument();
    expect(screen.getByText("Seis")).toBeVisible();
  });

  it("shows a retry option when spending fails to load", async () => {
    vi.mocked(transactionsApi.getCategoryBreakdown).mockRejectedValue(new Error("boom"));
    renderPage();
    expect(
      await screen.findByText("Não foi possível carregar as categorias."),
    ).toBeVisible();
  });

  it("shows an empty state instead of a zero-width proportion bar when there is no spending", async () => {
    vi.mocked(transactionsApi.getCategoryBreakdown).mockResolvedValue({
      month: "2026-08",
      classification: "outflow",
      currency_code: "BRL",
      total: "0.00",
      items: [],
    });
    renderPage();
    expect(await screen.findByText("Nenhuma saída neste período.")).toBeVisible();
  });
});

describe("Category breakdown navigation", () => {
  it("preserves the month on tab changes and builds category and uncategorized links", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Mercado");
    expect(screen.getByRole("button", { name: "Próximo mês" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "Mês anterior" }));
    await screen.findByText("Mercado");
    const calls = vi.mocked(transactionsApi.getCategoryBreakdown).mock.calls;
    const period = calls[calls.length - 1][1] as { from: string; to: string };
    await user.click(screen.getByRole("tab", { name: "Entradas" }));
    await screen.findByText("Total de entradas");
    expect(transactionsApi.getCategoryBreakdown).toHaveBeenLastCalledWith(expect.any(String), period, "inflow", expect.any(AbortSignal));
    const link = screen.getByRole("link", { name: /Mercado/ });
    const params = new URL(link.getAttribute("href")!, "http://localhost").searchParams;
    expect(params.get("classification")).toBe("inflow");
    expect(params.get("category_id")).toBe("cat-mercado");
    expect(params.get("date_from")).toBe(period.from);
    const uncategorized = screen.getByRole("link", { name: /Sem categoria/ });
    expect(uncategorized.getAttribute("href")).toContain("uncategorized=true");
    await user.click(screen.getByRole("button", { name: "Este mês" }));
    expect(screen.getByRole("button", { name: "Próximo mês" })).toBeEnabled();
    expect(screen.getByRole("tab", { name: "Entradas" })).toHaveAttribute("aria-selected", "true");
  });

  it("keeps controls available on errors and shows an inflow empty state after retry", async () => {
    vi.mocked(transactionsApi.getCategoryBreakdown).mockRejectedValue(new Error("offline"));
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Não foi possível carregar as categorias.");
    expect(screen.getByRole("button", { name: "Mês anterior" })).toBeEnabled();
    vi.mocked(transactionsApi.getCategoryBreakdown).mockResolvedValue({ ...spendingByCategory, classification: "inflow", total: "0.00", items: [] });
    await user.click(screen.getByRole("tab", { name: "Entradas" }));
    expect(await screen.findByText("Nenhuma entrada neste período.")).toBeVisible();
  });
});
