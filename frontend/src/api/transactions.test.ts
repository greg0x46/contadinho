import { describe, expect, it, vi } from "vitest";

import { formatMoney } from "../presentation/money";
import { transactionQuery, transactionResult, transactionJsonResponse } from "../test/transactionFixtures";
import { ApiError } from "./problems";
import { queryTransactions, setTransactionInclusion } from "./transactions";
import { transactionId } from "../test/transactionFixtures";

describe("transaction API boundary", () => {
  it("forwards cancellation and parses exact monetary strings", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(transactionJsonResponse());
    const controller = new AbortController();
    const result = await queryTransactions(transactionQuery, controller.signal);
    expect(result.items[0]?.amount).toBe("123.4500");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/transactions/query",
      expect.objectContaining({ method: "POST", signal: controller.signal }),
    );
  });

  it.each([
    ["monetary JSON numbers", { ...transactionResult, totals: [{ ...transactionResult.totals[0]!, inflow: 1 }] }],
    ["extra private fields", { ...transactionResult, raw_import_id: "private" }],
  ])("rejects %s", async (_name, payload) => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(transactionJsonResponse(payload));
    await expect(queryTransactions(transactionQuery)).rejects.toBeInstanceOf(ApiError);
  });

  it("handles application/problem+json", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      transactionJsonResponse(
        { type: "/problems/query", title: "Indisponível", status: 503, detail: "Tente novamente." },
        { status: 503, headers: { "content-type": "application/problem+json" } },
      ),
    );
    await expect(queryTransactions(transactionQuery)).rejects.toMatchObject({
      message: "Tente novamente.",
    });
  });
});

describe("transaction inclusion API boundary", () => {
  it("sends an explicit idempotent PUT and validates the confirmation", async () => {
    const response = {
      transaction_id: transactionId,
      state: "ignored",
      changed_at: "2026-07-30T13:00:00Z",
    };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      transactionJsonResponse(response),
    );
    await expect(setTransactionInclusion(transactionId, "ignored")).resolves.toEqual(response);
    expect(fetchMock).toHaveBeenCalledWith(
      `/api/transactions/${transactionId}/inclusion`,
      {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ state: "ignored" }),
      },
    );
  });

  it("rejects invalid confirmations and preserves problem details", async () => {
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        transactionJsonResponse({ transaction_id: transactionId, state: "ignored" }),
      )
      .mockResolvedValueOnce(
        transactionJsonResponse(
          { type: "/problems/write", title: "Indisponível", status: 503, detail: "Repita." },
          { status: 503, headers: { "content-type": "application/problem+json" } },
        ),
      );
    await expect(setTransactionInclusion(transactionId, "ignored")).rejects.toMatchObject({
      message: "A confirmação da decisão é inválida.",
    });
    await expect(setTransactionInclusion(transactionId, "ignored")).rejects.toMatchObject({
      message: "Repita.",
    });
  });
});

describe("string-only money formatting", () => {
  it.each([
    ["9007199254740993.1200", "BRL", "BRL 9.007.199.254.740.993,1200"],
    ["-0.50", "USD", "-USD 0,50"],
    ["3", "BRL", "BRL 3"],
  ])("preserves precision, sign, scale and trailing zeros", (value, currency, expected) => {
    expect(formatMoney(value, currency)).toBe(expected);
  });
});
