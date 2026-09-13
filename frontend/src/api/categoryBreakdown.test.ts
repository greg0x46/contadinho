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
    const controller = new AbortController();
    const signal = controller.signal;
    expect(await getCategoryBreakdown("America/Sao_Paulo", "2024-02", "inflow", signal)).toEqual(body);
    const [url, init] = fetchMock.mock.calls[0];
    const params = new URL(url, "http://localhost").searchParams;
    expect(Object.fromEntries(params)).toEqual({ timezone: "America/Sao_Paulo", month: "2024-02", classification: "inflow" });
    controller.abort();
    expect(init.signal.aborted).toBe(true);
  });

  it.each([
    [{ from: "2024-02-15", to: "2024-03-20" }, { date_from: "2024-02-15", date_to: "2024-03-20" }],
    [{ from: null, to: null }, { period: "all" }],
  ] as const)("sends a global period %j", async (period, query) => {
    const totals = { classification: body.classification, currency_code: body.currency_code, total: body.total, items: body.items };
    const response = { ...totals, date_from: period.from, date_to: period.to };
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(response), { headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();
    const signal = controller.signal;
    expect(await getCategoryBreakdown("UTC", period, "inflow", signal)).toEqual(response);
    expect(Object.fromEntries(new URL(fetchMock.mock.calls[0][0], "http://localhost").searchParams)).toEqual({ timezone: "UTC", classification: "inflow", ...query });
    controller.abort();
    expect(fetchMock.mock.calls[0][1].signal.aborted).toBe(true);
  });

  it.each([
    { date_from: "2024-02-30", date_to: "2024-03-20" },
    { date_from: "2024-03-20", date_to: "2024-02-15" },
    { date_from: null, date_to: "2024-03-20" },
    {},
  ])("rejects invalid range metadata %j", (dates) => {
    const totals = { classification: body.classification, currency_code: body.currency_code, total: body.total, items: body.items };
    expect(() => parseCategoryBreakdown({ ...totals, ...dates })).toThrow();
  });

  it("rejects invalid direction and monetary values", () => {
    expect(() => parseCategoryBreakdown({ ...body, classification: "transfer" })).toThrow();
    expect(() => parseCategoryBreakdown({ ...body, total: 25 })).toThrow();
  });
});
