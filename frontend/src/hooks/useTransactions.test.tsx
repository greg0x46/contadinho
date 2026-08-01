import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import * as transactionApi from "../api/transactions";
import { transactionResult } from "../test/transactionFixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { currentMonthFilters, useTransactions } from "./useTransactions";

vi.mock("../api/transactions");

describe("useTransactions", () => {
  it("disables automatic retry, forwards cancellation, and manually retries", async () => {
    vi.mocked(transactionApi.queryTransactions)
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(transactionResult);
    const { result } = renderHook(
      () => useTransactions(currentMonthFilters(new Date(2026, 6, 30)), "month", 1),
      { wrapper: QueryTestProvider },
    );
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(transactionApi.queryTransactions).toHaveBeenCalledTimes(1);
    await act(() => result.current.refetch());
    await waitFor(() => expect(result.current.data).toEqual(transactionResult));
  });

  it("preserves a confirmed same-key snapshot after a refetch error", async () => {
    vi.mocked(transactionApi.queryTransactions)
      .mockResolvedValueOnce(transactionResult)
      .mockRejectedValueOnce(new Error("offline"));
    const { result } = renderHook(
      () => useTransactions(currentMonthFilters(new Date(2026, 6, 30)), "month", 1),
      { wrapper: QueryTestProvider },
    );
    await waitFor(() => expect(result.current.data).toEqual(transactionResult));
    await act(() => result.current.refetch());
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.data).toEqual(transactionResult);
  });

  it("does not carry data across query identities", async () => {
    vi.mocked(transactionApi.queryTransactions)
      .mockResolvedValueOnce(transactionResult)
      .mockImplementationOnce(() => new Promise(() => undefined));
    const filters = currentMonthFilters(new Date(2026, 6, 30));
    const { result, rerender } = renderHook(
      ({ page }) => useTransactions(filters, "month", page),
      { wrapper: QueryTestProvider, initialProps: { page: 1 } },
    );
    await waitFor(() => expect(result.current.data).toEqual(transactionResult));
    rerender({ page: 2 });
    expect(result.current.data).toBeUndefined();
  });
});
