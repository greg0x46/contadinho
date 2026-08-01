import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import * as debtsApi from "../api/debts";
import type { Debt } from "../api/contracts";
import { useDebts } from "./useDebts";

vi.mock("../api/debts");

const debtId = "44444444-4444-4444-8444-444444444444";

const debt: Debt = {
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

function setup() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}

describe("useDebts", () => {
  it("lists debts from the backend", async () => {
    vi.mocked(debtsApi.listDebts).mockResolvedValue([debt]);
    const { wrapper } = setup();
    const { result } = renderHook(useDebts, { wrapper });
    await waitFor(() => expect(result.current.debts).toEqual([debt]));
  });

  it("invalidates the debts list after creating", async () => {
    vi.mocked(debtsApi.listDebts).mockResolvedValue([]);
    vi.mocked(debtsApi.createDebt).mockResolvedValue(debt);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(useDebts, { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(() =>
      result.current.createDebt({ name: debt.name, total_amount: 1000, initial_remaining_amount: null }),
    );

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["debts"] });
  });

  it("updates a debt and invalidates the list", async () => {
    vi.mocked(debtsApi.listDebts).mockResolvedValue([debt]);
    vi.mocked(debtsApi.updateDebt).mockResolvedValue({ ...debt, name: "Renomeada" });
    const { wrapper } = setup();
    const { result } = renderHook(useDebts, { wrapper });
    await waitFor(() => expect(result.current.debts).toEqual([debt]));

    await act(() =>
      result.current.updateDebt({ debtId, write: { name: "Renomeada", total_amount: 1000 } }),
    );

    expect(debtsApi.updateDebt).toHaveBeenCalledWith(debtId, {
      name: "Renomeada",
      total_amount: 1000,
    });
  });

  it("deletes a debt and invalidates the list", async () => {
    vi.mocked(debtsApi.listDebts).mockResolvedValue([debt]);
    vi.mocked(debtsApi.deleteDebt).mockResolvedValue(undefined);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(useDebts, { wrapper });
    await waitFor(() => expect(result.current.debts).toEqual([debt]));

    await act(() => result.current.deleteDebt(debtId));

    expect(debtsApi.deleteDebt).toHaveBeenCalledWith(debtId);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["debts"] });
  });
});
