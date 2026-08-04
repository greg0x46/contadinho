import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import * as receivablesApi from "../api/receivables";
import type { Receivable } from "../api/contracts";
import { useReceivables } from "./useReceivables";

vi.mock("../api/receivables");

const receivableId = "99999999-9999-4999-8999-999999999999";

const receivable: Receivable = {
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

function setup() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}

describe("useReceivables", () => {
  it("lists receivables from the backend", async () => {
    vi.mocked(receivablesApi.listReceivables).mockResolvedValue([receivable]);
    const { wrapper } = setup();
    const { result } = renderHook(useReceivables, { wrapper });
    await waitFor(() => expect(result.current.receivables).toEqual([receivable]));
  });

  it("invalidates the receivables list after creating", async () => {
    vi.mocked(receivablesApi.listReceivables).mockResolvedValue([]);
    vi.mocked(receivablesApi.createReceivable).mockResolvedValue(receivable);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(useReceivables, { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(() =>
      result.current.createReceivable({
        name: receivable.name,
        total_amount: 1000,
        initial_remaining_amount: null,
      }),
    );

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables"] });
  });

  it("updates a receivable and invalidates the list", async () => {
    vi.mocked(receivablesApi.listReceivables).mockResolvedValue([receivable]);
    vi.mocked(receivablesApi.updateReceivable).mockResolvedValue({ ...receivable, name: "Renomeada" });
    const { wrapper } = setup();
    const { result } = renderHook(useReceivables, { wrapper });
    await waitFor(() => expect(result.current.receivables).toEqual([receivable]));

    await act(() =>
      result.current.updateReceivable({
        receivableId,
        write: { name: "Renomeada", total_amount: 1000 },
      }),
    );

    expect(receivablesApi.updateReceivable).toHaveBeenCalledWith(receivableId, {
      name: "Renomeada",
      total_amount: 1000,
    });
  });

  it("deletes a receivable and invalidates the list", async () => {
    vi.mocked(receivablesApi.listReceivables).mockResolvedValue([receivable]);
    vi.mocked(receivablesApi.deleteReceivable).mockResolvedValue(undefined);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(useReceivables, { wrapper });
    await waitFor(() => expect(result.current.receivables).toEqual([receivable]));

    await act(() => result.current.deleteReceivable(receivableId));

    expect(receivablesApi.deleteReceivable).toHaveBeenCalledWith(receivableId);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables"] });
  });
});
