import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as debtsApi from "../api/debts";
import * as scenariosApi from "../api/scenarios";
import type { DebtDetail, EligibleTransaction } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { DebtDetailPage } from "./DebtDetailPage";

vi.mock("../api/debts");
vi.mock("../api/scenarios");

beforeEach(() => {
  vi.mocked(scenariosApi.listDebtScenarios).mockResolvedValue([]);
});

const debtId = "44444444-4444-4444-8444-444444444444";
const linkId = "66666666-6666-4666-8666-666666666666";
const transactionId = "77777777-7777-4777-8777-777777777777";

const debtDetail: DebtDetail = {
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
  links: [
    {
      id: linkId,
      transaction_id: transactionId,
      occurred_at: "2026-07-10T12:00:00Z",
      description: "Parcela 1",
      linked_amount: "200",
      current_amount: "200",
      linked_at: "2026-07-30T12:00:00Z",
    },
  ],
};

const candidate: EligibleTransaction = {
  id: "88888888-8888-4888-8888-888888888888",
  occurred_at: "2026-07-20T12:00:00Z",
  description: "Compra elegível",
  account_name: "Conta Corrente",
  effective_money: { value: "150", currency_code: "BRL" },
};

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={[`/dividas/${debtId}`]}>
      <QueryTestProvider>
        <Routes>
          <Route path="/dividas/:id" element={<DebtDetailPage />} />
        </Routes>
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("DebtDetailPage", () => {
  it("rejects a malformed debt id in the URL without calling the API", async () => {
    vi.mocked(debtsApi.listEligibleTransactions).mockResolvedValue([]);
    render(
      <MemoryRouter initialEntries={["/dividas/not-a-uuid"]}>
        <QueryTestProvider>
          <Routes>
            <Route path="/dividas/:id" element={<DebtDetailPage />} />
          </Routes>
        </QueryTestProvider>
      </MemoryRouter>,
    );
    expect(await screen.findByText("Endereço de dívida inválido")).toBeVisible();
    expect(debtsApi.getDebt).not.toHaveBeenCalled();
  });

  it("renders status as visible text and shows totals", async () => {
    vi.mocked(debtsApi.getDebt).mockResolvedValue(debtDetail);
    vi.mocked(debtsApi.listEligibleTransactions).mockResolvedValue([]);
    renderDetail();
    expect(await screen.findByText("Aberta")).toBeVisible();
    expect(screen.getByText("Parcela 1")).toBeVisible();
  });

  it("links a selected candidate transaction to the debt", async () => {
    const user = userEvent.setup();
    vi.mocked(debtsApi.getDebt).mockResolvedValue(debtDetail);
    vi.mocked(debtsApi.listEligibleTransactions).mockResolvedValue([candidate]);
    vi.mocked(debtsApi.createDebtLink).mockResolvedValue({
      id: "99999999-9999-4999-8999-999999999999",
      transaction_id: candidate.id,
      linked_amount: "150",
      linked_at: "2026-07-31T12:00:00Z",
    });
    renderDetail();

    const select = await screen.findByRole("combobox", {
      name: "Buscar transação para vincular",
    });
    await user.click(select);
    await user.type(select, "Compra");
    const option = await screen.findByText(/Compra elegível/);
    await user.click(option);
    await user.click(screen.getByRole("button", { name: "Vincular" }));

    await waitFor(() =>
      expect(debtsApi.createDebtLink).toHaveBeenCalledWith(debtId, candidate.id),
    );
  });

  it("unlinks a linked transaction after confirmation", async () => {
    const user = userEvent.setup();
    vi.mocked(debtsApi.getDebt).mockResolvedValue(debtDetail);
    vi.mocked(debtsApi.listEligibleTransactions).mockResolvedValue([]);
    vi.mocked(debtsApi.deleteDebtLink).mockResolvedValue(undefined);
    renderDetail();

    await user.click(await screen.findByRole("button", { name: "Desvincular" }));
    const confirm = await screen.findByRole("tooltip");
    await user.click(within(confirm).getByRole("button", { name: "Desvincular" }));

    await waitFor(() => expect(debtsApi.deleteDebtLink).toHaveBeenCalledWith(debtId, linkId));
  });

  it("shows an unavailable state when the debt cannot be fetched", async () => {
    vi.mocked(debtsApi.getDebt).mockRejectedValue(new Error("network down"));
    vi.mocked(debtsApi.listEligibleTransactions).mockResolvedValue([]);
    renderDetail();
    expect(
      await screen.findByText(/Não foi possível consultar esta dívida agora\./),
    ).toBeVisible();
  });
});
