import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as categoriesApi from "../api/categories";
import * as recurringCommitmentsApi from "../api/recurringCommitments";
import type { Category, RecurringCommitment } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { RecurringCommitmentsPage } from "./RecurringCommitmentsPage";

vi.mock("../api/recurringCommitments");
vi.mock("../api/categories");

const commitmentId = "33333333-3333-4333-8333-333333333333";
const categoryId = "44444444-4444-4444-8444-444444444444";

const category: Category = {
  id: categoryId,
  name: "Moradia",
  kind: "expense",
  is_active: true,
  icon: "home",
  color: "#495057",
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

const commitment: RecurringCommitment = {
  id: commitmentId,
  name: "Aluguel",
  kind: "expense",
  amount: "1500.00",
  category_id: categoryId,
  account_id: null,
  cadence: "monthly",
  day_of_month: 5,
  month_of_year: null,
  start_date: "2026-01-01",
  end_date: null,
  is_active: true,
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <RecurringCommitmentsPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("RecurringCommitmentsPage", () => {
  beforeEach(() => {
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([category]);
  });

  it("lists existing recurring commitments", async () => {
    vi.mocked(recurringCommitmentsApi.listRecurringCommitments).mockResolvedValue([commitment]);
    renderPage();
    expect(await screen.findByText("Aluguel")).toBeVisible();
    expect(screen.getByText("Moradia")).toBeVisible();
  });

  it("blocks saving without a name, amount or category", async () => {
    const user = userEvent.setup();
    vi.mocked(recurringCommitmentsApi.listRecurringCommitments).mockResolvedValue([]);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Novo compromisso" }));

    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um nome para o compromisso.")).toBeVisible();
    expect(recurringCommitmentsApi.createRecurringCommitment).not.toHaveBeenCalled();
  });

  it("creates a commitment with the entered fields", async () => {
    const user = userEvent.setup();
    vi.mocked(recurringCommitmentsApi.listRecurringCommitments).mockResolvedValue([]);
    vi.mocked(recurringCommitmentsApi.createRecurringCommitment).mockResolvedValue(commitment);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Novo compromisso" }));

    await user.type(screen.getByLabelText("Nome"), "Aluguel");
    await user.type(screen.getByLabelText("Valor"), "1500");
    await user.click(screen.getByRole("combobox", { name: "Categoria" }));
    await user.click(await screen.findByText("Moradia"));
    await user.type(screen.getByLabelText("Dia do mês"), "5");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(recurringCommitmentsApi.createRecurringCommitment).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Aluguel",
          amount: "1500.00",
          category_id: categoryId,
          day_of_month: 5,
          cadence: "monthly",
        }),
      ),
    );
  });

  it("toggles a commitment active state", async () => {
    const user = userEvent.setup();
    vi.mocked(recurringCommitmentsApi.listRecurringCommitments).mockResolvedValue([commitment]);
    vi.mocked(recurringCommitmentsApi.setRecurringCommitmentActive).mockResolvedValue({
      ...commitment,
      is_active: false,
    });
    renderPage();
    const toggle = await screen.findByRole("switch");
    await user.click(toggle);

    await waitFor(() =>
      expect(recurringCommitmentsApi.setRecurringCommitmentActive).toHaveBeenCalledWith(
        commitmentId,
        false,
      ),
    );
  });

  it("deletes a commitment after confirmation", async () => {
    const user = userEvent.setup();
    vi.mocked(recurringCommitmentsApi.listRecurringCommitments).mockResolvedValue([commitment]);
    vi.mocked(recurringCommitmentsApi.deleteRecurringCommitment).mockResolvedValue(undefined);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Excluir" }));
    const confirm = await screen.findByRole("tooltip");
    await user.click(within(confirm).getByRole("button", { name: "Excluir" }));

    await waitFor(() =>
      expect(recurringCommitmentsApi.deleteRecurringCommitment).toHaveBeenCalledWith(commitmentId),
    );
  });
});
