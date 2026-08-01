import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import * as transactionApi from "../api/transactions";
import { transactionId } from "../test/transactionFixtures";
import { useTransactionInclusion } from "./useTransactionInclusion";

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

describe("useTransactionInclusion", () => {
  it("cancels before PUT, avoids optimistic writes, stales the prefix and refetches active keys", async () => {
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
    const setData = vi.spyOn(client, "setQueryData");
    vi.mocked(transactionApi.setTransactionInclusion).mockImplementation(async () => {
      order.push("put");
      return { transaction_id: transactionId, state: "ignored", changed_at: "2026-07-30T13:00:00Z" };
    });
    const { result } = renderHook(useTransactionInclusion, { wrapper });

    act(() => result.current.setInclusion({ transactionId, state: "ignored" }));
    await waitFor(() =>
      expect(result.current.announcement).toBe("Transação ignorada. Totais atualizados."),
    );

    expect(order).toEqual(["cancel", "put", "invalidate", "refetch"]);
    expect(setData).not.toHaveBeenCalled();
    expect(client.invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["transactions"],
      refetchType: "none",
    });
    expect(client.refetchQueries).toHaveBeenCalledWith(
      {
        queryKey: ["transactions"],
        type: "active",
      },
      { throwOnError: true },
    );
  });

  it("distinguishes a failed PUT retry from refresh-only recovery", async () => {
    const { client, wrapper } = setup();
    vi.mocked(transactionApi.setTransactionInclusion)
      .mockRejectedValueOnce(new Error("write failed"))
      .mockResolvedValueOnce({
        transaction_id: transactionId,
        state: "ignored",
        changed_at: "2026-07-30T13:00:00Z",
      });
    vi.spyOn(client, "refetchQueries")
      .mockRejectedValueOnce(new Error("refresh failed"))
      .mockResolvedValueOnce(undefined);
    const { result } = renderHook(useTransactionInclusion, { wrapper });

    act(() => result.current.setInclusion({ transactionId, state: "ignored" }));
    await waitFor(() => expect(result.current.writeError).toBe("write failed"));
    act(() => result.current.retryWrite());
    await waitFor(() => expect(result.current.refreshError).toBe("refresh failed"));
    expect(transactionApi.setTransactionInclusion).toHaveBeenCalledTimes(2);
    await act(() => result.current.retryRefresh());
    expect(transactionApi.setTransactionInclusion).toHaveBeenCalledTimes(2);
    expect(result.current.announcement).toBe("Resultados atualizados.");
  });
});
