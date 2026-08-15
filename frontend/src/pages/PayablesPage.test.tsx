import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import * as payablesApi from "../api/payables";
import type { Payable } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { PayablesPage } from "./PayablesPage";

vi.mock("../api/payables");

const payableId = "44444444-4444-4444-8444-444444444444";

const openDebt: Payable = {
  id: payableId,
  kind: "debt",
  name: "Financiamento do carro",
  total_amount: "1000",
  starting_settled_amount: "0",
  settled_amount: "200",
  remaining_amount: "800",
  status: "open",
  link_count: 1,
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

const settledDebt: Payable = {
  ...openDebt,
  id: "55555555-5555-4555-8555-555555555555",
  name: "Cartão quitado",
  settled_amount: "1000",
  remaining_amount: "0",
  link_count: 0,
  status: "settled",
};

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <PayablesPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("PayablesPage", () => {
  it("renders status as visible text, not only color", async () => {
    vi.mocked(payablesApi.listPayables).mockResolvedValue([openDebt, settledDebt]);
    renderPage();
    expect(await screen.findByText("Aberta")).toBeVisible();
    expect(await screen.findByText("Quitada")).toBeVisible();
  });

  it("blocks saving a new debt without a name or with an invalid total amount", async () => {
    const user = userEvent.setup();
    vi.mocked(payablesApi.listPayables).mockResolvedValue([]);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova dívida" }));

    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um nome para a dívida.")).toBeVisible();
    expect(payablesApi.createPayable).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText("Nome"), "Nova dívida de teste");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um valor total maior que zero.")).toBeVisible();
    expect(payablesApi.createPayable).not.toHaveBeenCalled();
  });

  it("creates a debt with the entered name and total amount", async () => {
    const user = userEvent.setup();
    vi.mocked(payablesApi.listPayables).mockResolvedValue([]);
    vi.mocked(payablesApi.createPayable).mockResolvedValue(openDebt);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova dívida" }));
    await user.type(screen.getByLabelText("Nome"), "Financiamento do carro");
    await user.type(screen.getByLabelText("Valor total"), "1000");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(payablesApi.createPayable).toHaveBeenCalledWith({
        kind: "debt",
        name: "Financiamento do carro",
        total_amount: 1000,
        initial_remaining_amount: null,
      }),
    );
    expect(screen.queryByRole("button", { name: "Salvar" })).not.toBeInTheDocument();
  });

  it("edits an existing payable pre-filled with its current values, without the initial remaining field", async () => {
    const user = userEvent.setup();
    vi.mocked(payablesApi.listPayables).mockResolvedValue([openDebt]);
    vi.mocked(payablesApi.updatePayable).mockResolvedValue({ ...openDebt, name: "Renomeada" });
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Editar" }));
    const nameField = await screen.findByLabelText("Nome");
    expect(nameField).toHaveValue(openDebt.name);
    expect(screen.queryByLabelText("Valor restante inicial (opcional)")).not.toBeInTheDocument();

    await user.clear(nameField);
    await user.type(nameField, "Renomeada");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(payablesApi.updatePayable).toHaveBeenCalledWith(payableId, {
        name: "Renomeada",
        total_amount: 1000,
      }),
    );
  });

  it("warns how many links will be undone before deleting a payable with links", async () => {
    const user = userEvent.setup();
    vi.mocked(payablesApi.listPayables).mockResolvedValue([openDebt]);
    vi.mocked(payablesApi.deletePayable).mockResolvedValue(undefined);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Excluir" }));
    const confirm = await screen.findByRole("tooltip");
    expect(within(confirm).getByText(/1 transação vinculada será desfeita/)).toBeInTheDocument();
    await user.click(within(confirm).getByRole("button", { name: "Excluir" }));

    await waitFor(() => expect(payablesApi.deletePayable).toHaveBeenCalledWith(payableId));
  });
});
