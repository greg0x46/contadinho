import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import * as receivablesApi from "../api/receivables";
import type { ReceivableDetail } from "../api/contracts";
import { useReceivableDetail } from "./useReceivableDetail";

vi.mock("../api/receivables");

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
      description: "Recebimento",
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

describe("useReceivableDetail", () => {
  it("fetches the receivable detail and reports a fresh snapshot", async () => {
    vi.mocked(receivablesApi.getReceivable).mockResolvedValue(receivableDetail);
    vi.mocked(receivablesApi.listEligibleReceivableTransactions).mockResolvedValue([]);
    const { wrapper } = setup();
    const { result } = renderHook(() => useReceivableDetail(receivableId), { wrapper });

    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));
    expect(result.current.state.snapshot).toEqual(receivableDetail);
  });

  it("invalidates both the detail and the list after linking a transaction", async () => {
    vi.mocked(receivablesApi.getReceivable).mockResolvedValue(receivableDetail);
    vi.mocked(receivablesApi.listEligibleReceivableTransactions).mockResolvedValue([]);
    vi.mocked(receivablesApi.createReceivableLink).mockResolvedValue({
      id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
      transaction_id: transactionId,
      linked_amount: "50",
      linked_at: "2026-07-31T12:00:00Z",
    });
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useReceivableDetail(receivableId), { wrapper });
    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));

    await act(() => result.current.linkTransaction(transactionId));

    expect(receivablesApi.createReceivableLink).toHaveBeenCalledWith(receivableId, transactionId);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables", receivableId] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables"] });
  });

  it("invalidates both the detail and the list after unlinking a transaction", async () => {
    vi.mocked(receivablesApi.getReceivable).mockResolvedValue(receivableDetail);
    vi.mocked(receivablesApi.listEligibleReceivableTransactions).mockResolvedValue([]);
    vi.mocked(receivablesApi.deleteReceivableLink).mockResolvedValue(undefined);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useReceivableDetail(receivableId), { wrapper });
    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));

    await act(() => result.current.unlinkTransaction(linkId));

    expect(receivablesApi.deleteReceivableLink).toHaveBeenCalledWith(receivableId, linkId);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables", receivableId] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables"] });
  });
});
