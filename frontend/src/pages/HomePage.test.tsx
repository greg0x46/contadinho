import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as debtsApi from "../api/debts";
import * as transactionsApi from "../api/transactions";
import type { DebtTotalOwed, SpendingByCategory } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { HomePage } from "./HomePage";

vi.mock("../api/debts");
vi.mock("../api/transactions");

const totalOwed: DebtTotalOwed = {
  remaining_debts_total: "800.00",
  future_installments_total: "361.49",
  total_owed: "1161.49",
  currency_code: "BRL",
};

const spendingByCategory: SpendingByCategory = {
  month: "2026-08",
  currency_code: "BRL",
  total: "150.00",
  items: [
    { category_id: "cat-mercado", category_name: "Mercado", amount: "100.00" },
    { category_id: null, category_name: "Sem categoria", amount: "50.00" },
  ],
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
  vi.mocked(transactionsApi.getSpendingByCategory).mockResolvedValue(spendingByCategory);
});

describe("HomePage", () => {
  it("renders the total debt card once loaded", async () => {
    vi.mocked(debtsApi.getDebtTotalOwed).mockResolvedValue(totalOwed);
    renderPage();
    expect(await screen.findByText(/800,00/)).toBeVisible();
    expect(screen.getByText(/361,49/)).toBeVisible();
    expect(screen.getByText(/1\.161,49/)).toBeVisible();
  });

  it("shows a retry option when the total fails to load", async () => {
    vi.mocked(debtsApi.getDebtTotalOwed).mockRejectedValue(new Error("boom"));
    renderPage();
    expect(await screen.findByText("Não foi possível carregar o total de dívida.")).toBeVisible();
    expect(screen.getByRole("button", { name: "Tentar novamente" })).toBeVisible();
  });

  it("shows an empty state instead of a zero-width proportion bar when there is no debt", async () => {
    vi.mocked(debtsApi.getDebtTotalOwed).mockResolvedValue({
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
    vi.mocked(debtsApi.getDebtTotalOwed).mockResolvedValue(totalOwed);
  });

  it("renders each category's share of the month's spending once loaded", async () => {
    renderPage();
    expect(await screen.findByText("Mercado")).toBeVisible();
    expect(screen.getByText("Sem categoria")).toBeVisible();
    expect(screen.getByText(/150,00/)).toBeVisible();
    expect(screen.getByText(/100,00/)).toBeVisible();
    expect(screen.getByText("67%")).toBeVisible();
    expect(screen.getByText("33%")).toBeVisible();
  });

  it("folds categories beyond the top five into Outras categorias", async () => {
    vi.mocked(transactionsApi.getSpendingByCategory).mockResolvedValue({
      month: "2026-08",
      currency_code: "BRL",
      total: "600.00",
      items: [
        { category_id: "1", category_name: "Um", amount: "100.00" },
        { category_id: "2", category_name: "Dois", amount: "100.00" },
        { category_id: "3", category_name: "Tres", amount: "100.00" },
        { category_id: "4", category_name: "Quatro", amount: "100.00" },
        { category_id: "5", category_name: "Cinco", amount: "100.00" },
        { category_id: "6", category_name: "Seis", amount: "100.00" },
      ],
    });
    renderPage();
    expect(await screen.findByText("Cinco")).toBeVisible();
    expect(screen.getByText("Outras categorias")).toBeVisible();
    expect(screen.queryByText("Seis")).not.toBeInTheDocument();
  });

  it("shows a retry option when spending fails to load", async () => {
    vi.mocked(transactionsApi.getSpendingByCategory).mockRejectedValue(new Error("boom"));
    renderPage();
    expect(
      await screen.findByText("Não foi possível carregar os gastos por categoria."),
    ).toBeVisible();
  });

  it("shows an empty state instead of a zero-width proportion bar when there is no spending", async () => {
    vi.mocked(transactionsApi.getSpendingByCategory).mockResolvedValue({
      month: "2026-08",
      currency_code: "BRL",
      total: "0.00",
      items: [],
    });
    renderPage();
    expect(await screen.findByText("Nenhum gasto categorizado neste mês.")).toBeVisible();
  });
});
