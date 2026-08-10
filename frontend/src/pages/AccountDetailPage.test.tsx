import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import * as accountsApi from "../api/accounts";
import * as transactionsApi from "../api/transactions";
import type { TransactionQueryResult } from "../api/contracts";
import { ApiError } from "../api/problems";
import {
  accountBill,
  accountCard,
  bankAccount,
  bankAccountId,
  creditAccount,
  creditAccountId,
} from "../test/accountFixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { AccountDetailPage } from "./AccountDetailPage";

vi.mock("../api/accounts");
vi.mock("../api/transactions");

const emptyTransactions: TransactionQueryResult = {
  confirmed_at: "2026-08-01T12:00:00Z",
  stored_total: 0,
  page: { number: 1, size: 10, total_items: 0, total_pages: 0 },
  items: [],
  totals: [],
  groups: [],
  available_filters: { accounts: [], institutions: [], categories: [] },
};

function renderPage(id: string) {
  return render(
    <MemoryRouter initialEntries={[`/contas-e-cartoes/${id}`]}>
      <QueryTestProvider>
        <Routes>
          <Route path="/contas-e-cartoes/:id" element={<AccountDetailPage />} />
        </Routes>
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

const findSection = (name: string) =>
  screen.findByRole("heading", { name }, { timeout: 5000 });

describe("AccountDetailPage", () => {
  it("shows limit, cards, bills and transactions for a credit account", async () => {
    vi.mocked(accountsApi.getAccount).mockResolvedValue(creditAccount);
    vi.mocked(accountsApi.listAccountCards).mockResolvedValue([accountCard]);
    vi.mocked(accountsApi.listAccountBills).mockResolvedValue([accountBill]);
    vi.mocked(transactionsApi.queryTransactions).mockResolvedValue(emptyTransactions);
    renderPage(creditAccountId);

    expect(await findSection("Cartões desta conta")).toBeVisible();
    expect(screen.getByRole("heading", { name: "Faturas fechadas" })).toBeVisible();
    expect(screen.getByRole("heading", { name: "Últimas transações" })).toBeVisible();

    // Limit and usage live in the header card.
    expect(screen.getByText("Limite total")).toBeVisible();
    expect(screen.getByText("Limite disponível")).toBeVisible();
    expect(screen.getByText("24,7% do limite usado")).toBeVisible();

    expect(screen.getByText("•••• 1111")).toBeVisible();
    expect(
      screen.getByText(
        "A instituição disponibiliza apenas faturas já fechadas — a fatura em aberto aparece no saldo do cartão.",
      ),
    ).toBeVisible();
  });

  it("hides the cards and bills sections for a bank account", async () => {
    vi.mocked(accountsApi.getAccount).mockResolvedValue(bankAccount);
    vi.mocked(accountsApi.listAccountCards).mockResolvedValue([]);
    vi.mocked(accountsApi.listAccountBills).mockResolvedValue([]);
    vi.mocked(transactionsApi.queryTransactions).mockResolvedValue(emptyTransactions);
    renderPage(bankAccountId);

    expect(await findSection("Últimas transações")).toBeVisible();
    expect(screen.queryByRole("heading", { name: "Cartões desta conta" })).toBeNull();
    expect(screen.queryByRole("heading", { name: "Faturas fechadas" })).toBeNull();
    expect(screen.getByText("Conta Corrente")).toBeVisible();
  });

  it("reports a missing account", async () => {
    vi.mocked(accountsApi.getAccount).mockRejectedValue(
      new ApiError("not_found", "Conta não encontrada"),
    );
    vi.mocked(accountsApi.listAccountCards).mockResolvedValue([]);
    vi.mocked(accountsApi.listAccountBills).mockResolvedValue([]);
    vi.mocked(transactionsApi.queryTransactions).mockResolvedValue(emptyTransactions);
    renderPage(creditAccountId);

    expect(await screen.findByText("Conta não encontrada")).toBeVisible();
  });

  it("rejects an id that is not a uuid", () => {
    renderPage("nao-e-uuid");
    expect(screen.getByText("Endereço de conta inválido")).toBeVisible();
    expect(accountsApi.getAccount).not.toHaveBeenCalled();
  });

  it("lets the user state the closing day when the provider does not report it", async () => {
    const user = userEvent.setup();
    vi.mocked(accountsApi.getAccount).mockResolvedValue({
      ...creditAccount,
      closing_day: null,
      closing_day_source: null,
    });
    vi.mocked(accountsApi.listAccountCards).mockResolvedValue([]);
    vi.mocked(accountsApi.listAccountBills).mockResolvedValue([]);
    vi.mocked(transactionsApi.queryTransactions).mockResolvedValue(emptyTransactions);
    vi.mocked(accountsApi.setAccountClosingDay).mockResolvedValue({
      ...creditAccount,
      closing_day: 18,
      closing_day_source: "manual",
    });
    renderPage(creditAccountId);

    expect(await screen.findByText("Não informado", {}, { timeout: 5000 })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Definir" }));
    await user.type(screen.getByLabelText("Dia do mês"), "18");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(accountsApi.setAccountClosingDay).toHaveBeenCalledWith(creditAccountId, 18),
    );
  });

  it("shows the provider's closing day as a day of the month", async () => {
    vi.mocked(accountsApi.getAccount).mockResolvedValue(creditAccount);
    vi.mocked(accountsApi.listAccountCards).mockResolvedValue([]);
    vi.mocked(accountsApi.listAccountBills).mockResolvedValue([]);
    vi.mocked(transactionsApi.queryTransactions).mockResolvedValue(emptyTransactions);
    renderPage(creditAccountId);

    expect(await screen.findByText("Todo dia 3", {}, { timeout: 5000 })).toBeVisible();
    expect(screen.getByRole("button", { name: "Alterar" })).toBeVisible();
  });

  it("hands the closing day back to the provider when the user clears it", async () => {
    const user = userEvent.setup();
    vi.mocked(accountsApi.getAccount).mockResolvedValue({
      ...creditAccount,
      closing_day: 18,
      closing_day_source: "manual",
    });
    vi.mocked(accountsApi.listAccountCards).mockResolvedValue([]);
    vi.mocked(accountsApi.listAccountBills).mockResolvedValue([]);
    vi.mocked(transactionsApi.queryTransactions).mockResolvedValue(emptyTransactions);
    vi.mocked(accountsApi.setAccountClosingDay).mockResolvedValue(creditAccount);
    renderPage(creditAccountId);

    await user.click(await screen.findByRole("button", { name: "Alterar" }, { timeout: 5000 }));
    await user.click(screen.getByRole("button", { name: "Usar o dado da instituição" }));

    await waitFor(() =>
      expect(accountsApi.setAccountClosingDay).toHaveBeenCalledWith(creditAccountId, null),
    );
  });
});
