import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import * as payablesApi from "../api/payables";
import type { PayableDetail } from "../api/contracts";
import { usePayableDetail } from "./usePayableDetail";

vi.mock("../api/payables");

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

function setup() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}

describe("usePayableDetail", () => {
  it("fetches the payable detail and reports a fresh snapshot", async () => {
    vi.mocked(payablesApi.getPayable).mockResolvedValue(payableDetail);
    vi.mocked(payablesApi.listEligibleTransactions).mockResolvedValue([]);
    const { wrapper } = setup();
    const { result } = renderHook(() => usePayableDetail(payableId, "debt"), { wrapper });

    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));
    expect(result.current.state.snapshot).toEqual(payableDetail);
    expect(payablesApi.listEligibleTransactions).toHaveBeenCalledWith("debt", "", expect.anything());
  });

  it("invalidates both the detail and the list after linking a transaction", async () => {
    vi.mocked(payablesApi.getPayable).mockResolvedValue(payableDetail);
    vi.mocked(payablesApi.listEligibleTransactions).mockResolvedValue([]);
    vi.mocked(payablesApi.createPayableLink).mockResolvedValue({
      id: "99999999-9999-4999-8999-999999999999",
      transaction_id: transactionId,
      linked_amount: "50",
      linked_at: "2026-07-31T12:00:00Z",
    });
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => usePayableDetail(payableId, "debt"), { wrapper });
    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));

    await act(() => result.current.linkTransaction(transactionId));

    expect(payablesApi.createPayableLink).toHaveBeenCalledWith(payableId, transactionId);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["payables", "debt", payableId] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["payables"] });
  });

  it("invalidates both the detail and the list after unlinking a transaction", async () => {
    vi.mocked(payablesApi.getPayable).mockResolvedValue(payableDetail);
    vi.mocked(payablesApi.listEligibleTransactions).mockResolvedValue([]);
    vi.mocked(payablesApi.deletePayableLink).mockResolvedValue(undefined);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => usePayableDetail(payableId, "debt"), { wrapper });
    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));

    await act(() => result.current.unlinkTransaction(linkId));

    expect(payablesApi.deletePayableLink).toHaveBeenCalledWith(payableId, linkId);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["payables", "debt", payableId] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["payables"] });
  });
});
