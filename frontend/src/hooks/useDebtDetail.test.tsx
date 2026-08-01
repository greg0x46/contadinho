import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import * as debtsApi from "../api/debts";
import type { DebtDetail } from "../api/contracts";
import { useDebtDetail } from "./useDebtDetail";

vi.mock("../api/debts");

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

function setup() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}

describe("useDebtDetail", () => {
  it("fetches the debt detail and reports a fresh snapshot", async () => {
    vi.mocked(debtsApi.getDebt).mockResolvedValue(debtDetail);
    vi.mocked(debtsApi.listEligibleTransactions).mockResolvedValue([]);
    const { wrapper } = setup();
    const { result } = renderHook(() => useDebtDetail(debtId), { wrapper });

    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));
    expect(result.current.state.snapshot).toEqual(debtDetail);
  });

  it("invalidates both the detail and the list after linking a transaction", async () => {
    vi.mocked(debtsApi.getDebt).mockResolvedValue(debtDetail);
    vi.mocked(debtsApi.listEligibleTransactions).mockResolvedValue([]);
    vi.mocked(debtsApi.createDebtLink).mockResolvedValue({
      id: "99999999-9999-4999-8999-999999999999",
      transaction_id: transactionId,
      linked_amount: "50",
      linked_at: "2026-07-31T12:00:00Z",
    });
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useDebtDetail(debtId), { wrapper });
    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));

    await act(() => result.current.linkTransaction(transactionId));

    expect(debtsApi.createDebtLink).toHaveBeenCalledWith(debtId, transactionId);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["debts", debtId] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["debts"] });
  });

  it("invalidates both the detail and the list after unlinking a transaction", async () => {
    vi.mocked(debtsApi.getDebt).mockResolvedValue(debtDetail);
    vi.mocked(debtsApi.listEligibleTransactions).mockResolvedValue([]);
    vi.mocked(debtsApi.deleteDebtLink).mockResolvedValue(undefined);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useDebtDetail(debtId), { wrapper });
    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));

    await act(() => result.current.unlinkTransaction(linkId));

    expect(debtsApi.deleteDebtLink).toHaveBeenCalledWith(debtId, linkId);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["debts", debtId] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["debts"] });
  });
});
