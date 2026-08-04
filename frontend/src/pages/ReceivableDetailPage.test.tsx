import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as receivablesApi from "../api/receivables";
import * as scenariosApi from "../api/scenarios";
import type { EligibleTransaction, ReceivableDetail } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { ReceivableDetailPage } from "./ReceivableDetailPage";

vi.mock("../api/receivables");
vi.mock("../api/scenarios");

beforeEach(() => {
  vi.mocked(scenariosApi.listReceivableScenarios).mockResolvedValue([]);
});

const receivableId = "99999999-9999-4999-8999-999999999999";
const linkId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const transactionId = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";

const receivableDetail: ReceivableDetail = {
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
  id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
  occurred_at: "2026-07-20T12:00:00Z",
  description: "Recebimento elegível",
  account_name: "Conta Corrente",
  effective_money: { value: "150", currency_code: "BRL" },
};

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={[`/contas-a-receber/${receivableId}`]}>
      <QueryTestProvider>
        <Routes>
          <Route path="/contas-a-receber/:id" element={<ReceivableDetailPage />} />
        </Routes>
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("ReceivableDetailPage", () => {
  it("rejects a malformed receivable id in the URL without calling the API", async () => {
    vi.mocked(receivablesApi.listEligibleReceivableTransactions).mockResolvedValue([]);
    render(
      <MemoryRouter initialEntries={["/contas-a-receber/not-a-uuid"]}>
        <QueryTestProvider>
          <Routes>
            <Route path="/contas-a-receber/:id" element={<ReceivableDetailPage />} />
          </Routes>
        </QueryTestProvider>
      </MemoryRouter>,
    );
    expect(await screen.findByText("Endereço de conta a receber inválido")).toBeVisible();
    expect(receivablesApi.getReceivable).not.toHaveBeenCalled();
  });

  it("renders status as visible text and shows totals", async () => {
    vi.mocked(receivablesApi.getReceivable).mockResolvedValue(receivableDetail);
    vi.mocked(receivablesApi.listEligibleReceivableTransactions).mockResolvedValue([]);
    renderDetail();
    expect(await screen.findByText("Aberta")).toBeVisible();
    expect(await screen.findByText(/Parcela 1/)).toBeVisible();
  });

  it("links a selected candidate transaction to the receivable", async () => {
    const user = userEvent.setup();
    vi.mocked(receivablesApi.getReceivable).mockResolvedValue(receivableDetail);
    vi.mocked(receivablesApi.listEligibleReceivableTransactions).mockResolvedValue([candidate]);
    vi.mocked(receivablesApi.createReceivableLink).mockResolvedValue({
      id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
      transaction_id: candidate.id,
      linked_amount: "150",
      linked_at: "2026-07-31T12:00:00Z",
    });
    renderDetail();

    await user.click(await screen.findByRole("button", { name: "Adicionar recebimento" }));
    const select = await screen.findByRole("combobox", {
      name: "Buscar transação para vincular",
    });
    await user.click(select);
    await user.type(select, "Recebimento");
    const option = await screen.findByText(/Recebimento elegível/);
    await user.click(option);
    await user.click(screen.getByRole("button", { name: "Vincular" }));

    await waitFor(() =>
      expect(receivablesApi.createReceivableLink).toHaveBeenCalledWith(receivableId, candidate.id),
    );
  });

  it("unlinks a linked transaction after confirmation", async () => {
    const user = userEvent.setup();
    vi.mocked(receivablesApi.getReceivable).mockResolvedValue(receivableDetail);
    vi.mocked(receivablesApi.listEligibleReceivableTransactions).mockResolvedValue([]);
    vi.mocked(receivablesApi.deleteReceivableLink).mockResolvedValue(undefined);
    renderDetail();

    await user.click(await screen.findByRole("button", { name: "Desvincular" }));
    const confirm = await screen.findByRole("tooltip");
    await user.click(within(confirm).getByRole("button", { name: "Desvincular" }));

    await waitFor(() =>
      expect(receivablesApi.deleteReceivableLink).toHaveBeenCalledWith(receivableId, linkId),
    );
  });

  it("shows an unavailable state when the receivable cannot be fetched", async () => {
    vi.mocked(receivablesApi.getReceivable).mockRejectedValue(new Error("network down"));
    vi.mocked(receivablesApi.listEligibleReceivableTransactions).mockResolvedValue([]);
    renderDetail();
    expect(
      await screen.findByText(/Não foi possível consultar esta conta a receber agora\./),
    ).toBeVisible();
  });
});
