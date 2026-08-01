import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import * as transactionApi from "../api/transactions";
import { categoryId, transactionId } from "../test/transactionFixtures";
import { useTransactionCategory } from "./useTransactionCategory";

vi.mock("../api/transactions");

function setup() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}

describe("useTransactionCategory", () => {
  it("cancels before PUT, invalidates the transactions prefix and refetches active keys", async () => {
    const { client, wrapper } = setup();
    const order: string[] = [];
    vi.spyOn(client, "cancelQueries").mockImplementation(async () => {
      order.push("cancel");
    });
    vi.spyOn(client, "invalidateQueries").mockImplementation(async () => {
      order.push("invalidate");
    });
    vi.spyOn(client, "refetchQueries").mockImplementation(async () => {
      order.push("refetch");
    });
    vi.mocked(transactionApi.setTransactionCategory).mockImplementation(async () => {
      order.push("put");
      return {
        transaction_id: transactionId,
        category_id: categoryId,
        origin: "manual",
        changed_at: "2026-07-31T00:00:00Z",
      };
    });
    const { result } = renderHook(useTransactionCategory, { wrapper });

    act(() => result.current.setCategory({ transactionId, categoryId }));
    await waitFor(() =>
      expect(result.current.announcement).toBe("Categoria atualizada. Totais atualizados."),
    );

    expect(order).toEqual(["cancel", "put", "invalidate", "refetch"]);
    expect(client.invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["transactions"],
      refetchType: "none",
    });
    expect(client.refetchQueries).toHaveBeenCalledWith(
      { queryKey: ["transactions"], type: "active" },
      { throwOnError: true },
    );
  });

  it("distinguishes a failed PUT retry from refresh-only recovery", async () => {
    const { client, wrapper } = setup();
    vi.mocked(transactionApi.setTransactionCategory)
      .mockRejectedValueOnce(new Error("write failed"))
      .mockResolvedValueOnce({
        transaction_id: transactionId,
        category_id: categoryId,
        origin: "manual",
        changed_at: "2026-07-31T00:00:00Z",
      });
    vi.spyOn(client, "refetchQueries")
      .mockRejectedValueOnce(new Error("refresh failed"))
      .mockResolvedValueOnce(undefined);
    const { result } = renderHook(useTransactionCategory, { wrapper });

    act(() => result.current.setCategory({ transactionId, categoryId }));
    await waitFor(() => expect(result.current.writeError).toBe("write failed"));
    act(() => result.current.retryWrite());
    await waitFor(() => expect(result.current.refreshError).toBe("refresh failed"));
    expect(transactionApi.setTransactionCategory).toHaveBeenCalledTimes(2);
    await act(() => result.current.retryRefresh());
    expect(transactionApi.setTransactionCategory).toHaveBeenCalledTimes(2);
    expect(result.current.announcement).toBe("Categoria atualizada. Totais atualizados.");
  });
});
