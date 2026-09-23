import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import * as accountsApi from "../api/accounts";
import { ApiError } from "../api/problems";
import { bankAccount, creditAccount } from "../test/accountFixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { AccountsPage } from "./AccountsPage";

vi.mock("../api/accounts");

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <AccountsPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

// The headings render before the query resolves, so waiting on one doesn't
// mean the rows are in yet.
const findRow = (text: string) => screen.findByText(text, {}, { timeout: 5000 });

describe("AccountsPage", () => {
  it("splits bank accounts and credit cards into their own sections", async () => {
    vi.mocked(accountsApi.listAccounts).mockResolvedValue([bankAccount, creditAccount]);
    renderPage();

    expect(await findRow("Conta Corrente")).toBeVisible();
    expect(screen.getByRole("heading", { name: "Contas bancárias" })).toBeVisible();
    expect(screen.getByRole("heading", { name: "Cartões de crédito" })).toBeVisible();
    expect(screen.getByText("Cartão Platinum")).toBeVisible();
  });

  it("shows each account's balance and the card's limit usage", async () => {
    vi.mocked(accountsApi.listAccounts).mockResolvedValue([bankAccount, creditAccount]);
    renderPage();

    await findRow("24,7% do limite usado");
    // Each figure shows twice: once in its row, once in the summary.
    expect(screen.getAllByText("R$ 2.500,00").length).toBeGreaterThan(0);
    expect(screen.getAllByText("R$ 1.234,56").length).toBeGreaterThan(0);
    expect(screen.getByText("24,7% do limite usado")).toBeVisible();
  });

  it("treats an account with no type as a bank account", async () => {
    vi.mocked(accountsApi.listAccounts).mockResolvedValue([
      { ...bankAccount, account_type: null, name: "Conta sem tipo" },
    ]);
    renderPage();

    expect(await findRow("Conta sem tipo")).toBeVisible();
    expect(screen.getByText("Nenhum cartão de crédito sincronizado ainda.")).toBeVisible();
  });

  it("offers a retry when the accounts cannot be loaded", async () => {
    const user = userEvent.setup();
    vi.mocked(accountsApi.listAccounts).mockRejectedValue(
      new ApiError("response", "Não foi possível carregar as contas."),
    );
    renderPage();

    await user.click(await screen.findByRole("button", { name: "Tentar novamente" }));
    await waitFor(() => expect(accountsApi.listAccounts).toHaveBeenCalledTimes(2));
  });
});
