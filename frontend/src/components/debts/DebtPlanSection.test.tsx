import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import * as scenariosApi from "../../api/scenarios";
import type { Scenario, ScenarioDetail } from "../../api/contracts";
import { QueryTestProvider } from "../../test/QueryTestProvider";
import { DebtPlanSection } from "./DebtPlanSection";

vi.mock("../../api/scenarios");

const debtId = "44444444-4444-4444-8444-444444444444";
const scenarioId = "55555555-5555-4555-8555-555555555555";

const scenarioSummary: Scenario = {
  id: scenarioId,
  kind: "debt_plan",
  name: "Plano de pagamento",
  debt_id: debtId,
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

function detailWith(deviation: string): ScenarioDetail {
  return {
    ...scenarioSummary,
    accumulated_deviation: deviation,
    transactions: [
      {
        id: "66666666-6666-4666-8666-666666666666",
        scenario_id: scenarioId,
        description: "Parcela 1/3",
        amount: "333.33",
        projected_at: "2026-01-01",
        category: null,
        status: "atrasada",
      },
    ],
  };
}

function renderSection(detail: ScenarioDetail) {
  vi.mocked(scenariosApi.listDebtScenarios).mockResolvedValue([scenarioSummary]);
  vi.mocked(scenariosApi.getScenario).mockResolvedValue(detail);
  return render(
    <QueryTestProvider>
      <DebtPlanSection debtId={debtId} links={[]} />
    </QueryTestProvider>,
  );
}

describe("DebtPlanSection accumulated deviation banner", () => {
  it("shows a behind-plan message for a positive deviation", async () => {
    renderSection(detailWith("40.00"));
    expect(await screen.findByText(/Atrasado.*em relação ao plano até hoje\./)).toBeVisible();
  });

  it("shows an ahead-of-plan message for a negative deviation", async () => {
    renderSection(detailWith("-40.00"));
    expect(await screen.findByText(/Adiantado.*em relação ao plano até hoje\./)).toBeVisible();
  });

  it("shows an on-track message for a zero deviation", async () => {
    renderSection(detailWith("0.00"));
    expect(await screen.findByText("Em dia com o plano de pagamento até hoje.")).toBeVisible();
  });
});

describe("DebtPlanSection generation form", () => {
  const emptyPlan: ScenarioDetail = { ...scenarioSummary, transactions: [], accumulated_deviation: "0.00" };

  it("generates by number of installments with the selected cadence", async () => {
    const user = userEvent.setup();
    vi.mocked(scenariosApi.generateInstallments).mockResolvedValue([]);
    renderSection(emptyPlan);

    await user.click(await screen.findByText("Semanal"));
    await user.click(screen.getByRole("button", { name: "Gerar parcelas" }));

    await waitFor(() =>
      expect(scenariosApi.generateInstallments).toHaveBeenCalledWith(scenarioId, {
        cadence: "semanal",
        months: 6,
      }),
    );
  });

  it("generates by installment value when that mode is selected", async () => {
    const user = userEvent.setup();
    vi.mocked(scenariosApi.generateInstallments).mockResolvedValue([]);
    renderSection(emptyPlan);

    await user.click(await screen.findByRole("radio", { name: "Valor da parcela" }));
    await user.type(screen.getByLabelText("Valor de cada parcela"), "400");
    await user.click(screen.getByRole("button", { name: "Gerar parcelas" }));

    await waitFor(() =>
      expect(scenariosApi.generateInstallments).toHaveBeenCalledWith(scenarioId, {
        cadence: "mensal",
        installment_amount: 400,
      }),
    );
  });
});
