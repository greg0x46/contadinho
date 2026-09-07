import { afterEach, describe, expect, it, vi } from "vitest";
import { getCategoryBreakdown } from "./transactions";
import { parseCategoryBreakdown } from "./contracts";

const body = {
  month: "2024-02", classification: "inflow", currency_code: "BRL", total: "25.00",
  items: [{ category_id: null, category_name: "Sem categoria", category_icon: "", category_color: "", amount: "25.00", source: "real" }],
};
afterEach(() => vi.unstubAllGlobals());

describe("category breakdown API", () => {
  it("parses the response and sends the selected period, direction and cancellation signal", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    const signal = new AbortController().signal;
    expect(await getCategoryBreakdown("America/Sao_Paulo", "2024-02", "inflow", signal)).toEqual(body);
    const [url, init] = fetchMock.mock.calls[0];
    const params = new URL(url, "http://localhost").searchParams;
    expect(Object.fromEntries(params)).toEqual({ timezone: "America/Sao_Paulo", month: "2024-02", classification: "inflow" });
    expect(init.signal).toBe(signal);
  });

  it("rejects invalid direction and monetary values", () => {
    expect(() => parseCategoryBreakdown({ ...body, classification: "transfer" })).toThrow();
    expect(() => parseCategoryBreakdown({ ...body, total: 25 })).toThrow();
  });
});
