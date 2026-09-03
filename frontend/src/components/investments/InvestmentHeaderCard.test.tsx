import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { Investment, InvestmentTransaction } from "../../api/contracts";
import { InvestmentHeaderCard } from "./InvestmentHeaderCard";

const investment: Investment = {
  id: "11111111-1111-4111-8111-111111111111",
  external_id: "inv-1",
  source_display_name: "Banco de Exemplo",
  investment_type: "EQUITY",
  subtype: null,
  name: "BBAS3",
  balance: "0",
  currency_code: "BRL",
  quantity: null,
  value: null,
  amount: null,
  amount_profit: null,
  amount_withdrawal: null,
  rate: null,
  rate_type: null,
  fixed_annual_rate: null,
  annual_rate: null,
  last_twelve_months_rate: null,
  issuer: null,
  due_date: null,
  as_of_date: null,
  provider_updated_at: null,
  yield_value: null,
  yield_source: null,
  yield_unavailable_reason: null,
};

const movements: InvestmentTransaction[] = [
  {
    id: "22222222-2222-4222-8222-222222222222",
    external_id: "invtx-1",
    movement_type: "BUY",
    direction: "inflow",
    quantity: "80",
    value: "20.00",
    amount: "1600.00",
    occurred_at: "2026-01-10T00:00:00Z",
    trade_date: null,
  },
  {
    id: "33333333-3333-4333-8333-333333333333",
    external_id: "invtx-2",
    movement_type: "SELL",
    direction: "outflow",
    quantity: "120",
    value: "24.17",
    amount: "2900.00",
    occurred_at: "2026-05-10T00:00:00Z",
    trade_date: null,
  },
];

describe("InvestmentHeaderCard", () => {
  // The card used to contradict itself: "Rendimento: Histórico incompleto"
  // directly above a confident "Aportes líquidos" netted from the very
  // history the backend had just rejected. Only the backend has the evidence
  // for that call (120 cotas sold against 80 bought), so the card follows it.
  it("hides the net contributed figure when the backend rejected the history", () => {
    render(
      <InvestmentHeaderCard
        investment={{ ...investment, yield_unavailable_reason: "historico_incompleto" }}
        transactions={movements}
      />,
    );

    expect(screen.getByText("Histórico incompleto")).toBeVisible();
    expect(screen.queryByText("Aportes líquidos")).not.toBeInTheDocument();
  });

  it("still shows it when the history is complete enough to net", () => {
    render(
      <InvestmentHeaderCard
        investment={{ ...investment, yield_value: "10.00", yield_source: "calculado" }}
        transactions={movements}
      />,
    );

    expect(screen.getByText("Aportes líquidos")).toBeVisible();
  });
});
