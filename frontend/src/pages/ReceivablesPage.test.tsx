import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import * as receivablesApi from "../api/receivables";
import type { Receivable } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { ReceivablesPage } from "./ReceivablesPage";

vi.mock("../api/receivables");

const receivableId = "99999999-9999-4999-8999-999999999999";

const openReceivable: Receivable = {
  id: receivableId,
  name: "Empréstimo para Ana",
  total_amount: "1000",
  starting_received_amount: "0",
  received_amount: "200",
  remaining_amount: "800",
  status: "open",
  link_count: 1,
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

const settledReceivable: Receivable = {
  ...openReceivable,
  id: "88888888-8888-4888-8888-888888888888",
  name: "Empréstimo quitado",
  received_amount: "1000",
  remaining_amount: "0",
  link_count: 0,
  status: "settled",
};

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <ReceivablesPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("ReceivablesPage", () => {
  it("renders status as visible text, not only color", async () => {
    vi.mocked(receivablesApi.listReceivables).mockResolvedValue([openReceivable, settledReceivable]);
    renderPage();
    expect(await screen.findByText("Aberta")).toBeVisible();
    expect(await screen.findByText("Recebida")).toBeVisible();
  });

  it("blocks saving a new receivable without a name or with an invalid total amount", async () => {
    const user = userEvent.setup();
    vi.mocked(receivablesApi.listReceivables).mockResolvedValue([]);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova conta a receber" }));

    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um nome para a conta a receber.")).toBeVisible();
    expect(receivablesApi.createReceivable).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText("Nome"), "Nova conta de teste");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um valor total maior que zero.")).toBeVisible();
    expect(receivablesApi.createReceivable).not.toHaveBeenCalled();
  });

  it("creates a receivable with the entered name and total amount", async () => {
    const user = userEvent.setup();
    vi.mocked(receivablesApi.listReceivables).mockResolvedValue([]);
    vi.mocked(receivablesApi.createReceivable).mockResolvedValue(openReceivable);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova conta a receber" }));
    await user.type(screen.getByLabelText("Nome"), "Empréstimo para Ana");
    await user.type(screen.getByLabelText("Valor total"), "1000");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(receivablesApi.createReceivable).toHaveBeenCalledWith({
        name: "Empréstimo para Ana",
        total_amount: 1000,
        initial_remaining_amount: null,
      }),
    );
    expect(screen.queryByRole("button", { name: "Salvar" })).not.toBeInTheDocument();
  });

  it("edits an existing receivable pre-filled with its current values, without the initial remaining field", async () => {
    const user = userEvent.setup();
    vi.mocked(receivablesApi.listReceivables).mockResolvedValue([openReceivable]);
    vi.mocked(receivablesApi.updateReceivable).mockResolvedValue({ ...openReceivable, name: "Renomeada" });
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Editar" }));
    const nameField = await screen.findByLabelText("Nome");
    expect(nameField).toHaveValue(openReceivable.name);
    expect(screen.queryByLabelText("Valor restante inicial (opcional)")).not.toBeInTheDocument();

    await user.clear(nameField);
    await user.type(nameField, "Renomeada");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(receivablesApi.updateReceivable).toHaveBeenCalledWith(receivableId, {
        name: "Renomeada",
        total_amount: 1000,
      }),
    );
  });

  it("warns how many links will be undone before deleting a receivable with links", async () => {
    const user = userEvent.setup();
    vi.mocked(receivablesApi.listReceivables).mockResolvedValue([openReceivable]);
    vi.mocked(receivablesApi.deleteReceivable).mockResolvedValue(undefined);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Excluir" }));
    const confirm = await screen.findByRole("tooltip");
    expect(within(confirm).getByText(/1 transação vinculada será desfeita/)).toBeInTheDocument();
    await user.click(within(confirm).getByRole("button", { name: "Excluir" }));

    await waitFor(() => expect(receivablesApi.deleteReceivable).toHaveBeenCalledWith(receivableId));
  });
});
