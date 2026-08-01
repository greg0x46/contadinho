import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ScenarioTransaction } from "../../api/contracts";
import { DebtPlanTable } from "./DebtPlanTable";

const overdue: ScenarioTransaction = {
  id: "11111111-1111-4111-8111-111111111111",
  scenario_id: "22222222-2222-4222-8222-222222222222",
  description: "Parcela 1/3",
  amount: "333.33",
  projected_at: "2026-01-01",
  category: null,
  status: "atrasada",
};

const upcoming: ScenarioTransaction = {
  ...overdue,
  id: "33333333-3333-4333-8333-333333333333",
  description: "Parcela 2/3",
  status: "projetada",
};

describe("DebtPlanTable", () => {
  it("renders a status tag per installment", () => {
    render(
      <DebtPlanTable installments={[overdue, upcoming]} deletingId={null} onDelete={vi.fn()} onAllocate={vi.fn()} />,
    );
    expect(screen.getByText("Atrasada")).toBeVisible();
    expect(screen.getByText("Projeção")).toBeVisible();
  });

  it("shows the formatted date and amount", () => {
    render(<DebtPlanTable installments={[overdue]} deletingId={null} onDelete={vi.fn()} onAllocate={vi.fn()} />);
    expect(screen.getByText("01/01/2026")).toBeVisible();
    expect(screen.getByText(/R\$\s*333,33/)).toBeVisible();
  });
});
