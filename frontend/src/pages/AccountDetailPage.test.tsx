import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import * as accountsApi from "../api/accounts";
import * as categoriesApi from "../api/categories";
import * as transactionsApi from "../api/transactions";
import { bankAccount, bankAccountId } from "../test/accountFixtures";
import { transactionResult } from "../test/transactionFixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { AccountDetailPage } from "./AccountDetailPage";

vi.mock("../api/accounts");
vi.mock("../api/transactions");
vi.mock("../api/categories");

function renderPage(accountId: string) {
  return render(
    <MemoryRouter initialEntries={[`/contas-e-cartoes/${accountId}`]}>
      <QueryTestProvider>
        <Routes>
          <Route path="/contas-e-cartoes/:id" element={<AccountDetailPage />} />
        </Routes>
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

const findHeading = (name: string) => screen.findByRole("heading", { name }, { timeout: 5000 });

describe("AccountDetailPage", () => {
  it("never shows Pluggy's proxy connector name, even when the provider reported it", async () => {
    vi.mocked(accountsApi.getAccount).mockResolvedValue({
      ...bankAccount,
      institution: "MeuPluggy",
      institution_name: null,
    });
    vi.mocked(accountsApi.listAccountCards).mockResolvedValue([]);
    vi.mocked(accountsApi.listAccountBills).mockResolvedValue([]);
    vi.mocked(accountsApi.listAccounts).mockResolvedValue([]);
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([]);
    vi.mocked(transactionsApi.queryTransactions).mockResolvedValue({
      ...transactionResult,
      items: [],
      groups: [],
    });

    const { container } = renderPage(bankAccountId);

    // Falls back to the account's own name, per accountHeaderTitle.
    await findHeading(bankAccount.name!);
    expect(container.textContent).not.toContain("MeuPluggy");
  });

  it("shows the short institution name, the balance in full, and reuses TransactionRow for recent activity", async () => {
    vi.mocked(accountsApi.getAccount).mockResolvedValue({
      ...bankAccount,
      institution_name: "Nu Pagamentos S.A. - Instituição de Pagamento",
    });
    vi.mocked(accountsApi.listAccountCards).mockResolvedValue([]);
    vi.mocked(accountsApi.listAccountBills).mockResolvedValue([]);
    vi.mocked(accountsApi.listAccounts).mockResolvedValue([]);
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([]);
    vi.mocked(transactionsApi.queryTransactions).mockResolvedValue(transactionResult);

    renderPage(bankAccountId);

    await findHeading("Nu Pagamentos");
    expect(screen.getByText("Saldo disponível")).toBeVisible();
    expect(screen.getByText("R$ 2.500,00")).toBeVisible();
    expect(screen.getByText("Últimas transações")).toBeVisible();
    expect(screen.getByRole("link", { name: "Ver todas" })).toBeVisible();
    // The recent-transactions row is TransactionRow itself, not a
    // page-specific one — same click-to-open button as Transações.
    expect(
      await screen.findByRole("button", { name: /Ver detalhes de Mercado/ }),
    ).toBeVisible();
  });
});
