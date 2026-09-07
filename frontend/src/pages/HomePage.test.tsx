import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as payablesApi from "../api/payables";
import * as timelineApi from "../api/timeline";
import * as transactionsApi from "../api/transactions";
import type {
  PayableTotalOwed,
  PayableTotalToReceive,
  CategoryBreakdown,
  TimelineResponse,
} from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { HomePage } from "./HomePage";

vi.mock("../api/payables");
vi.mock("../api/timeline");
vi.mock("../api/transactions");

const totalOwed: PayableTotalOwed = {
  remaining_debts_total: "800.00",
  future_installments_total: "361.49",
  total_owed: "1161.49",
  currency_code: "BRL",
};

const totalToReceive: PayableTotalToReceive = {
  remaining_receivables_total: "500.00",
  total_to_receive: "500.00",
  currency_code: "BRL",
};

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
      { date: "2026-08-15", balance: "1900.00", inflow: "0.00", outflow: "0.00", lowest_tier: "realizado" },
      { date: "2026-11-30", balance: "1500.00", inflow: "0.00", outflow: "400.00", lowest_tier: "projetado" },
    ],
    entries: [],
    starting_balance: "1900.00",
    lowest_balance: {
      date: "2026-11-30",
      balance: "1500.00",
      inflow: "0.00",
      outflow: "400.00",
      lowest_tier: "projetado",
    },
    first_negative: null,
  },
  monthly_breakdown: [],
  category_breakdown: [],
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
        <HomePage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.mocked(transactionsApi.getCategoryBreakdown).mockResolvedValue(spendingByCategory);
  vi.mocked(payablesApi.getPayableTotalToReceive).mockResolvedValue(totalToReceive);
  vi.mocked(timelineApi.getTimeline).mockResolvedValue(projection);
});

describe("TotalReceivableCard", () => {
  beforeEach(() => {
    vi.mocked(payablesApi.getPayableTotalOwed).mockResolvedValue(totalOwed);
  });

  it("renders the total receivable card once loaded", async () => {
    renderPage();
    const title = await screen.findByText("Total a receber");
    const card = title.closest<HTMLElement>(".ant-card");
    if (!card) throw new Error("Card de recebíveis não encontrado.");
    expect(await within(card).findByText(/500,00/)).toBeVisible();
  });

  it("shows a retry option when the total fails to load", async () => {
    vi.mocked(payablesApi.getPayableTotalToReceive).mockRejectedValue(new Error("boom"));
    renderPage();
    expect(await screen.findByText("Não foi possível carregar o total a receber.")).toBeVisible();
  });
});

describe("HomePage", () => {
  it("renders a compact projection summary on the Home dashboard", async () => {
    vi.mocked(payablesApi.getPayableTotalOwed).mockResolvedValue(totalOwed);
    renderPage();
    expect(await screen.findByText("Próximos 3 meses")).toBeVisible();
    expect(screen.getByText("Projeção de saldo")).toBeVisible();
    expect(screen.getByText("Saldo hoje")).toBeVisible();
  });

  it("keeps the projection horizon options in the card header", async () => {
    vi.mocked(payablesApi.getPayableTotalOwed).mockResolvedValue(totalOwed);
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByText("Fim do mês")).toBeVisible();
    expect(screen.getByText("3 meses")).toBeVisible();
    expect(screen.getByText("6 meses")).toBeVisible();
    expect(screen.getByText("12 meses")).toBeVisible();

    await user.click(screen.getByText("6 meses"));
    expect(await screen.findByText("Próximos 6 meses")).toBeVisible();
  });

  it("keeps current cards on the left and projection metrics above its chart", async () => {
    vi.mocked(payablesApi.getPayableTotalOwed).mockResolvedValue(totalOwed);
    renderPage();
    await screen.findByText("Próximos 3 meses");

    const layout = document.querySelector(".dashboard-layout");
    expect(layout?.firstElementChild).toHaveClass("dashboard-current-summary");
    expect(layout?.lastElementChild).toHaveClass("dashboard-widget");

    const summary = document.querySelector(".projection-summary");
    expect(summary?.firstElementChild).toHaveClass("projection-summary-intro");
    expect(summary?.lastElementChild).toHaveClass("timeline-chart");
  });

  it("shows a retry option when the projection fails to load", async () => {
    vi.mocked(payablesApi.getPayableTotalOwed).mockResolvedValue(totalOwed);
    vi.mocked(timelineApi.getTimeline).mockRejectedValue(new Error("boom"));
    renderPage();
    expect(await screen.findByText("Não foi possível carregar a projeção.")).toBeVisible();
  });

  it("renders the total debt card once loaded", async () => {
    vi.mocked(payablesApi.getPayableTotalOwed).mockResolvedValue(totalOwed);
    renderPage();
    expect(await screen.findByText(/800,00/)).toBeVisible();
    expect(screen.getByText(/361,49/)).toBeVisible();
    expect(screen.getByText(/1\.161,49/)).toBeVisible();
  });

  it("shows a retry option when the total fails to load", async () => {
    vi.mocked(payablesApi.getPayableTotalOwed).mockRejectedValue(new Error("boom"));
    renderPage();
    expect(await screen.findByText("Não foi possível carregar o total de dívida.")).toBeVisible();
    expect(screen.getByRole("button", { name: "Tentar novamente" })).toBeVisible();
  });

  it("shows an empty state instead of a zero-width proportion bar when there is no debt", async () => {
    vi.mocked(payablesApi.getPayableTotalOwed).mockResolvedValue({
      remaining_debts_total: "0.00",
      future_installments_total: "0.00",
      total_owed: "0.00",
      currency_code: "BRL",
    });
    renderPage();
    expect(await screen.findByText("Nenhuma dívida em aberto no momento.")).toBeVisible();
    expect(screen.queryByText("Dívidas restantes")).not.toBeInTheDocument();
  });
});

describe("SpendingByCategoryCard", () => {
  beforeEach(() => {
    vi.mocked(payablesApi.getPayableTotalOwed).mockResolvedValue(totalOwed);
  });

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
    expect(await screen.findByText("Nenhuma saída neste mês.")).toBeVisible();
  });
});

describe("Category breakdown navigation", () => {
  beforeEach(() => {
    vi.mocked(payablesApi.getPayableTotalOwed).mockResolvedValue(totalOwed);
  });

  it("preserves the month on tab changes and builds category and uncategorized links", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Mercado");
    expect(screen.getByRole("button", { name: "Próximo mês" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Mês anterior" }));
    await screen.findByText("Mercado");
    const calls = vi.mocked(transactionsApi.getCategoryBreakdown).mock.calls;
    const month = calls[calls.length - 1][1];
    await user.click(screen.getByRole("tab", { name: "Entradas" }));
    await screen.findByText("Total de entradas");
    expect(transactionsApi.getCategoryBreakdown).toHaveBeenLastCalledWith(expect.any(String), month, "inflow", expect.any(AbortSignal));
    const link = screen.getByRole("link", { name: /Mercado/ });
    const params = new URL(link.getAttribute("href")!, "http://localhost").searchParams;
    expect(params.get("classification")).toBe("inflow");
    expect(params.get("category_id")).toBe("cat-mercado");
    expect(params.get("date_from")).toBe(month + "-01");
    const uncategorized = screen.getByRole("link", { name: /Sem categoria/ });
    expect(uncategorized.getAttribute("href")).toContain("uncategorized=true");
    await user.click(screen.getByRole("button", { name: "Voltar ao mês atual" }));
    expect(screen.getByRole("button", { name: "Próximo mês" })).toBeDisabled();
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
    expect(await screen.findByText("Nenhuma entrada neste mês.")).toBeVisible();
  });
});
