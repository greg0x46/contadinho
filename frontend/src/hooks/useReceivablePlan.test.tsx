import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import * as scenariosApi from "../api/scenarios";
import type { Scenario, ScenarioDetail } from "../api/contracts";
import { useReceivablePlan } from "./useReceivablePlan";

vi.mock("../api/scenarios");

const receivableId = "99999999-9999-4999-8999-999999999999";
const scenarioId = "55555555-5555-4555-8555-555555555555";

const scenarioSummary: Scenario = {
  id: scenarioId,
  kind: "receivable_plan",
  name: "Plano de recebimento",
  debt_id: null,
  receivable_id: receivableId,
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

const scenarioDetail: ScenarioDetail = {
  ...scenarioSummary,
  transactions: [
    {
      id: "66666666-6666-4666-8666-666666666666",
      scenario_id: scenarioId,
      description: "Parcela 1/3",
      amount: "333.33",
      projected_at: "2026-09-01",
      category: null,
      status: "projetada",
      realizations: [],
    },
  ],
  accumulated_deviation: "0.00",
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

describe("useReceivablePlan", () => {
  it("reports no plan when the receivable has no scenarios yet", async () => {
    vi.mocked(scenariosApi.listReceivableScenarios).mockResolvedValue([]);
    const { wrapper } = setup();
    const { result } = renderHook(() => useReceivablePlan(receivableId), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.plan).toBeNull();
  });

  it("loads the first scenario's detail once the list resolves", async () => {
    vi.mocked(scenariosApi.listReceivableScenarios).mockResolvedValue([scenarioSummary]);
    vi.mocked(scenariosApi.getScenario).mockResolvedValue(scenarioDetail);
    const { wrapper } = setup();
    const { result } = renderHook(() => useReceivablePlan(receivableId), { wrapper });

    await waitFor(() => expect(result.current.plan).not.toBeNull());
    expect(scenariosApi.getScenario).toHaveBeenCalledWith(scenarioId, expect.anything());
    expect(result.current.plan?.transactions).toHaveLength(1);
  });

  it("invalidates the scenarios query after generating installments", async () => {
    vi.mocked(scenariosApi.listReceivableScenarios).mockResolvedValue([scenarioSummary]);
    vi.mocked(scenariosApi.getScenario).mockResolvedValue(scenarioDetail);
    vi.mocked(scenariosApi.generateInstallments).mockResolvedValue(scenarioDetail.transactions);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useReceivablePlan(receivableId), { wrapper });
    await waitFor(() => expect(result.current.plan).not.toBeNull());

    await act(() => result.current.generateInstallments({ cadence: "mensal", months: 3 }));

    expect(scenariosApi.generateInstallments).toHaveBeenCalledWith(scenarioId, {
      cadence: "mensal",
      months: 3,
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables", receivableId, "scenarios"] });
  });

  it("creates a plan and invalidates the scenarios query", async () => {
    vi.mocked(scenariosApi.listReceivableScenarios).mockResolvedValue([]);
    vi.mocked(scenariosApi.createReceivableScenario).mockResolvedValue({
      ...scenarioSummary,
      transactions: [],
      accumulated_deviation: "0.00",
    });
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useReceivablePlan(receivableId), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(() => result.current.createPlan("Plano de recebimento"));

    expect(scenariosApi.createReceivableScenario).toHaveBeenCalledWith(receivableId, {
      name: "Plano de recebimento",
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables", receivableId, "scenarios"] });
  });

  it("allocates a receivable link to an installment and invalidates the scenarios query", async () => {
    const transactionId = scenarioDetail.transactions[0].id;
    vi.mocked(scenariosApi.listReceivableScenarios).mockResolvedValue([scenarioSummary]);
    vi.mocked(scenariosApi.getScenario).mockResolvedValue(scenarioDetail);
    vi.mocked(scenariosApi.createRealization).mockResolvedValue({
      ...scenarioDetail.transactions[0],
      status: "paga",
    });
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useReceivablePlan(receivableId), { wrapper });
    await waitFor(() => expect(result.current.plan).not.toBeNull());

    const linkId = "77777777-7777-4777-8777-777777777777";
    await act(() =>
      result.current.allocateRealization({
        transactionId,
        write: { receivable_link_id: linkId, allocated_amount: 333.33 },
      }),
    );

    expect(scenariosApi.createRealization).toHaveBeenCalledWith(scenarioId, transactionId, {
      receivable_link_id: linkId,
      allocated_amount: 333.33,
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables", receivableId, "scenarios"] });
  });

  it("deallocates a realization from an installment and invalidates the scenarios query", async () => {
    const transactionId = scenarioDetail.transactions[0].id;
    vi.mocked(scenariosApi.listReceivableScenarios).mockResolvedValue([scenarioSummary]);
    vi.mocked(scenariosApi.getScenario).mockResolvedValue(scenarioDetail);
    vi.mocked(scenariosApi.deleteRealization).mockResolvedValue(undefined);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useReceivablePlan(receivableId), { wrapper });
    await waitFor(() => expect(result.current.plan).not.toBeNull());

    const realizationId = "88888888-8888-4888-8888-888888888888";
    await act(() => result.current.deallocateRealization({ transactionId, realizationId }));

    expect(scenariosApi.deleteRealization).toHaveBeenCalledWith(scenarioId, transactionId, realizationId);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables", receivableId, "scenarios"] });
  });

  it("readjusts installments and invalidates the scenarios query", async () => {
    vi.mocked(scenariosApi.listReceivableScenarios).mockResolvedValue([scenarioSummary]);
    vi.mocked(scenariosApi.getScenario).mockResolvedValue(scenarioDetail);
    vi.mocked(scenariosApi.readjustInstallments).mockResolvedValue(scenarioDetail.transactions);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useReceivablePlan(receivableId), { wrapper });
    await waitFor(() => expect(result.current.plan).not.toBeNull());

    await act(() => result.current.readjust({ strategy: "redistribuir" }));

    expect(scenariosApi.readjustInstallments).toHaveBeenCalledWith(scenarioId, { strategy: "redistribuir" });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["receivables", receivableId, "scenarios"] });
  });
});
