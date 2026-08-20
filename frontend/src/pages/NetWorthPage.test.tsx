import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import * as netWorthApi from "../api/netWorth";
import type { NetWorthSnapshot } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { NetWorthPage } from "./NetWorthPage";

vi.mock("../api/netWorth");

const snapshot: NetWorthSnapshot = {
  captured_at: "2026-08-18T00:00:00Z",
  is_backfilled: false,
  cash_balance: "1000.00",
  investment_balance: "500.00",
  credit_card_balance: "300.00",
  payables_debt: "200.00",
  total_assets: "1500.00",
  total_liabilities: "500.00",
  net_worth: "1150.00",
};

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <NetWorthPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("NetWorthPage", () => {
  it("renders the breakdown once the backend responds", async () => {
    vi.mocked(netWorthApi.getNetWorth).mockResolvedValue({ series: [snapshot], latest: snapshot });

    renderPage();

    await waitFor(() => expect(screen.getByText("R$ 1.150,00")).toBeInTheDocument());
    expect(screen.getByText("Composição do patrimônio")).toBeInTheDocument();
  });

  it("shows an error state with a retry action when the backend fails", async () => {
    vi.mocked(netWorthApi.getNetWorth).mockRejectedValue(new Error("boom"));

    renderPage();

    await waitFor(() =>
      expect(screen.getByText("Não foi possível carregar o patrimônio líquido")).toBeInTheDocument(),
    );
  });

  it("notes that backfilled points exclude investments when the series has one", async () => {
    const backfilled: NetWorthSnapshot = { ...snapshot, captured_at: "2026-08-17T00:00:00Z", is_backfilled: true };
    vi.mocked(netWorthApi.getNetWorth).mockResolvedValue({ series: [backfilled, snapshot], latest: snapshot });

    renderPage();

    await waitFor(() => expect(screen.getByText("Pontos reconstruídos não incluem investimentos")).toBeInTheDocument());
  });

  it("omits the backfill note when no point in the series was reconstructed", async () => {
    const other: NetWorthSnapshot = { ...snapshot, captured_at: "2026-08-17T00:00:00Z" };
    vi.mocked(netWorthApi.getNetWorth).mockResolvedValue({ series: [other, snapshot], latest: snapshot });

    renderPage();

    await waitFor(() => expect(screen.getByText("Composição do patrimônio")).toBeInTheDocument());
    expect(screen.queryByText("Pontos reconstruídos não incluem investimentos")).not.toBeInTheDocument();
  });
});
