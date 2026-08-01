import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import * as debtsApi from "../api/debts";
import type { Debt } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { DebtsPage } from "./DebtsPage";

vi.mock("../api/debts");

const debtId = "44444444-4444-4444-8444-444444444444";

const openDebt: Debt = {
  id: debtId,
  name: "Financiamento do carro",
  total_amount: "1000",
  starting_paid_amount: "0",
  paid_amount: "200",
  remaining_amount: "800",
  status: "open",
  link_count: 1,
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

const settledDebt: Debt = {
  ...openDebt,
  id: "55555555-5555-4555-8555-555555555555",
  name: "Cartão quitado",
  paid_amount: "1000",
  remaining_amount: "0",
  link_count: 0,
  status: "settled",
};

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <DebtsPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("DebtsPage", () => {
  it("renders status as visible text, not only color", async () => {
    vi.mocked(debtsApi.listDebts).mockResolvedValue([openDebt, settledDebt]);
    renderPage();
    expect(await screen.findByText("Aberta")).toBeVisible();
    expect(await screen.findByText("Quitada")).toBeVisible();
  });

  it("blocks saving a new debt without a name or with an invalid total amount", async () => {
    const user = userEvent.setup();
    vi.mocked(debtsApi.listDebts).mockResolvedValue([]);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova dívida" }));

    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um nome para a dívida.")).toBeVisible();
    expect(debtsApi.createDebt).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText("Nome"), "Nova dívida de teste");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um valor total maior que zero.")).toBeVisible();
    expect(debtsApi.createDebt).not.toHaveBeenCalled();
  });

  it("creates a debt with the entered name and total amount", async () => {
    const user = userEvent.setup();
    vi.mocked(debtsApi.listDebts).mockResolvedValue([]);
    vi.mocked(debtsApi.createDebt).mockResolvedValue(openDebt);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova dívida" }));
    await user.type(screen.getByLabelText("Nome"), "Financiamento do carro");
    await user.type(screen.getByLabelText("Valor total"), "1000");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(debtsApi.createDebt).toHaveBeenCalledWith({
        name: "Financiamento do carro",
        total_amount: 1000,
        initial_remaining_amount: null,
      }),
    );
    expect(screen.queryByRole("button", { name: "Salvar" })).not.toBeInTheDocument();
  });

  it("edits an existing debt pre-filled with its current values, without the initial remaining field", async () => {
    const user = userEvent.setup();
    vi.mocked(debtsApi.listDebts).mockResolvedValue([openDebt]);
    vi.mocked(debtsApi.updateDebt).mockResolvedValue({ ...openDebt, name: "Renomeada" });
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Editar" }));
    const nameField = await screen.findByLabelText("Nome");
    expect(nameField).toHaveValue(openDebt.name);
    expect(screen.queryByLabelText("Valor restante inicial (opcional)")).not.toBeInTheDocument();

    await user.clear(nameField);
    await user.type(nameField, "Renomeada");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(debtsApi.updateDebt).toHaveBeenCalledWith(debtId, {
        name: "Renomeada",
        total_amount: 1000,
      }),
    );
  });

  it("warns how many links will be undone before deleting a debt with links", async () => {
    const user = userEvent.setup();
    vi.mocked(debtsApi.listDebts).mockResolvedValue([openDebt]);
    vi.mocked(debtsApi.deleteDebt).mockResolvedValue(undefined);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Excluir" }));
    const confirm = await screen.findByRole("tooltip");
    expect(within(confirm).getByText(/1 transação vinculada será desfeita/)).toBeInTheDocument();
    await user.click(within(confirm).getByRole("button", { name: "Excluir" }));

    await waitFor(() => expect(debtsApi.deleteDebt).toHaveBeenCalledWith(debtId));
  });
});
