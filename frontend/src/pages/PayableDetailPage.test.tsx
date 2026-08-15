import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as payablesApi from "../api/payables";
import * as scenariosApi from "../api/scenarios";
import type { PayableDetail, EligibleTransaction } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { PayableDetailPage } from "./PayableDetailPage";

vi.mock("../api/payables");
vi.mock("../api/scenarios");

beforeEach(() => {
  vi.mocked(scenariosApi.listPayableScenarios).mockResolvedValue([]);
});

const payableId = "44444444-4444-4444-8444-444444444444";
const linkId = "66666666-6666-4666-8666-666666666666";
const transactionId = "77777777-7777-4777-8777-777777777777";

const payableDetail: PayableDetail = {
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
    <MemoryRouter initialEntries={[`/pendencias/${payableId}?kind=debt`]}>
      <QueryTestProvider>
        <Routes>
          <Route path="/pendencias/:id" element={<PayableDetailPage />} />
        </Routes>
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("PayableDetailPage", () => {
  it("rejects a malformed id in the URL without calling the API", async () => {
    vi.mocked(payablesApi.listEligibleTransactions).mockResolvedValue([]);
    render(
      <MemoryRouter initialEntries={["/pendencias/not-a-uuid?kind=debt"]}>
        <QueryTestProvider>
          <Routes>
            <Route path="/pendencias/:id" element={<PayableDetailPage />} />
          </Routes>
        </QueryTestProvider>
      </MemoryRouter>,
    );
    expect(await screen.findByText("Endereço inválido")).toBeVisible();
    expect(payablesApi.getPayable).not.toHaveBeenCalled();
  });

  it("renders status as visible text and shows totals", async () => {
    vi.mocked(payablesApi.getPayable).mockResolvedValue(payableDetail);
    vi.mocked(payablesApi.listEligibleTransactions).mockResolvedValue([]);
    renderDetail();
    expect(await screen.findByText("Aberta")).toBeVisible();
    expect(await screen.findByText(/Parcela 1/)).toBeVisible();
  });

  it("links a selected candidate transaction to the payable", async () => {
    const user = userEvent.setup();
    vi.mocked(payablesApi.getPayable).mockResolvedValue(payableDetail);
    vi.mocked(payablesApi.listEligibleTransactions).mockResolvedValue([candidate]);
    vi.mocked(payablesApi.createPayableLink).mockResolvedValue({
      id: "99999999-9999-4999-8999-999999999999",
      transaction_id: candidate.id,
      linked_amount: "150",
      linked_at: "2026-07-31T12:00:00Z",
    });
    renderDetail();

    await user.click(await screen.findByRole("button", { name: "Adicionar pagamento" }));
    const select = await screen.findByRole("combobox", {
      name: "Buscar transação para vincular",
    });
    await user.click(select);
    await user.type(select, "Compra");
    const option = await screen.findByText(/Compra elegível/);
    await user.click(option);
    await user.click(screen.getByRole("button", { name: "Vincular" }));

    await waitFor(() =>
      expect(payablesApi.createPayableLink).toHaveBeenCalledWith(payableId, candidate.id),
    );
  });

  it("unlinks a linked transaction after confirmation", async () => {
    const user = userEvent.setup();
    vi.mocked(payablesApi.getPayable).mockResolvedValue(payableDetail);
    vi.mocked(payablesApi.listEligibleTransactions).mockResolvedValue([]);
    vi.mocked(payablesApi.deletePayableLink).mockResolvedValue(undefined);
    renderDetail();

    await user.click(await screen.findByRole("button", { name: "Desvincular" }));
    const confirm = await screen.findByRole("tooltip");
    await user.click(within(confirm).getByRole("button", { name: "Desvincular" }));

    await waitFor(() => expect(payablesApi.deletePayableLink).toHaveBeenCalledWith(payableId, linkId));
  });

  it("shows an unavailable state when the payable cannot be fetched", async () => {
    vi.mocked(payablesApi.getPayable).mockRejectedValue(new Error("network down"));
    vi.mocked(payablesApi.listEligibleTransactions).mockResolvedValue([]);
    renderDetail();
    expect(
      await screen.findByText(/Não foi possível consultar esta pendência agora\./),
    ).toBeVisible();
  });
});
