import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { TimelineResponse } from "../../api/contracts";
import { queryKeys } from "../../api/queryKeys";
import * as timelineApi from "../../api/timeline";
import { DailyAllowanceCard } from "./DailyAllowanceCard";

vi.mock("../../api/timeline");

function response(lowest: string): TimelineResponse {
  const point = { date: "2026-10-25", balance: lowest, inflow: "0.00", outflow: "0.00", lowest_tier: "projetado" };
  return {
    base: {
      points: [point],
      entries: [],
      starting_balance: "5000.00",
      lowest_balance: point,
      first_negative: null,
    },
    period_totals: { income: "0.00", expense: "0.00", result: "0.00" },
    simulation: null,
    scenario_impacts: [],
  } as unknown as TimelineResponse;
}

function renderCard() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <DailyAllowanceCard />
    </QueryClientProvider>,
  );
  return client;
}

// The figure is split across the amount and its unit, so match the strip's whole text.
const heroText = () => document.querySelector(".summary-strip-value")?.textContent?.replace(/\s/g, " ");

describe("DailyAllowanceCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Only Date: react-query and testing-library still need real timers.
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 9, 22, 10, 0, 0));
  });

  afterEach(() => vi.useRealTimers());

  it("splits the lowest balance from today to month-end over the days left", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValue(response("855.00"));
    renderCard();

    await waitFor(() => expect(heroText()).toBe("R$ 85,50 / dia"));
    expect(screen.getByText("Disponível por dia")).toBeVisible();
    expect(screen.getByText("Disponível para gastar até o fim do mês")).toBeVisible();
    expect(timelineApi.getTimeline).toHaveBeenCalledWith(
      { referenceDate: "2026-10-22", from: "2026-10-22", to: "2026-10-31" },
      expect.anything(),
    );
  });

  it("shows zero when the month dips below zero", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValue(response("-300.00"));
    renderCard();

    await waitFor(() => expect(heroText()).toBe("R$ 0,00 / dia"));
  });

  it("shows a skeleton while loading", () => {
    vi.mocked(timelineApi.getTimeline).mockReturnValue(new Promise(() => {}));
    renderCard();

    expect(screen.getByRole("status", { name: "Carregando o gasto diário disponível" })).toBeInTheDocument();
  });

  it("offers a retry when the timeline fails", async () => {
    vi.mocked(timelineApi.getTimeline)
      .mockRejectedValueOnce(new Error("boom"))
      .mockResolvedValue(response("855.00"));
    renderCard();

    const retry = await screen.findByRole("button", { name: "Tentar novamente" });
    expect(screen.getByText("Não foi possível calcular o gasto diário disponível.")).toBeVisible();
    await userEvent.click(retry);
    await waitFor(() => expect(heroText()).toBe("R$ 85,50 / dia"));
  });

  it("follows the timeline when a write invalidates it, storing nothing", async () => {
    vi.mocked(timelineApi.getTimeline).mockResolvedValueOnce(response("855.00")).mockResolvedValue(response("500.00"));
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const client = renderCard();

    await waitFor(() => expect(heroText()).toBe("R$ 85,50 / dia"));
    await act(() => client.invalidateQueries({ queryKey: queryKeys.timeline }));
    await waitFor(() => expect(heroText()).toBe("R$ 50,00 / dia"));
    expect(setItem).not.toHaveBeenCalled();
    setItem.mockRestore();
  });
});
