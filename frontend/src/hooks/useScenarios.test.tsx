import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import * as scenariosApi from "../api/scenarios";
import type { Scenario } from "../api/contracts";
import { useScenarios } from "./useScenarios";

vi.mock("../api/scenarios");

const scenario: Scenario = {
  id: "11111111-1111-4111-8111-111111111111",
  kind: "recurring",
  name: "Moradia",
  payable_id: null,
  is_active: false,
  is_accounting_source: false,
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

// Activating or deactivating a scenario changes what the unified projector
// includes in every timeline series — the Home dashboard's saldo/entradas/
// saídas, the Relatório Financeiro's chart. A cache left stale here is a
// widget that silently keeps showing the pre-toggle numbers.
describe("useScenarios", () => {
  it("invalidates the timeline query after toggling a scenario active", async () => {
    vi.mocked(scenariosApi.listScenarios).mockResolvedValue([scenario]);
    vi.mocked(scenariosApi.setScenarioActive).mockResolvedValue({ ...scenario, is_active: true });
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useScenarios(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(() => result.current.toggleScenario({ scenarioId: scenario.id, isActive: true }));

    expect(scenariosApi.setScenarioActive).toHaveBeenCalledWith(scenario.id, true);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["timeline"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["timeline-data-range"] });
  });
});
