import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as scenariosApi from "../api/scenarios";
import type { Scenario, ScenarioDetail } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { ScenariosPage } from "./ScenariosPage";

vi.mock("../api/scenarios");

const scenarioId = "33333333-3333-4333-8333-333333333333";

const scenario: Scenario = {
  id: scenarioId,
  kind: "standalone",
  name: "Viagem",
  payable_id: null,
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

const scenarioDetail: ScenarioDetail = {
  ...scenario,
  transactions: [],
  accumulated_deviation: "0.00",
};

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <ScenariosPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("ScenariosPage", () => {
  beforeEach(() => {
    vi.mocked(scenariosApi.getScenario).mockResolvedValue(scenarioDetail);
  });

  it("lists existing standalone scenarios", async () => {
    vi.mocked(scenariosApi.listStandaloneScenarios).mockResolvedValue([scenario]);
    renderPage();
    expect(await screen.findByText("Viagem")).toBeVisible();
  });

  it("shows an empty state when there are no scenarios", async () => {
    vi.mocked(scenariosApi.listStandaloneScenarios).mockResolvedValue([]);
    renderPage();
    expect(await screen.findByText("Nenhum cenário criado ainda.")).toBeVisible();
  });

  it("blocks saving a new scenario without a name", async () => {
    const user = userEvent.setup();
    vi.mocked(scenariosApi.listStandaloneScenarios).mockResolvedValue([]);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Novo cenário" }));

    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um nome para o cenário.")).toBeVisible();
    expect(scenariosApi.createStandaloneScenario).not.toHaveBeenCalled();
  });

  it("creates a scenario with the entered name", async () => {
    const user = userEvent.setup();
    vi.mocked(scenariosApi.listStandaloneScenarios).mockResolvedValue([]);
    vi.mocked(scenariosApi.createStandaloneScenario).mockResolvedValue(scenarioDetail);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Novo cenário" }));
    await user.type(screen.getByLabelText("Nome"), "Viagem");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(scenariosApi.createStandaloneScenario).toHaveBeenCalledWith({ name: "Viagem" }),
    );
  });

  it("adds a hypothetical transaction to a scenario", async () => {
    const user = userEvent.setup();
    vi.mocked(scenariosApi.listStandaloneScenarios).mockResolvedValue([scenario]);
    vi.mocked(scenariosApi.createScenarioTransaction).mockResolvedValue({
      id: "44444444-4444-4444-8444-444444444444",
      scenario_id: scenarioId,
      description: "Passagem",
      amount: "-800.00",
      projected_at: "2026-12-01",
      category: null,
      status: "projetada",
      realizations: [],
    });
    renderPage();
    await screen.findByText("Viagem");

    await user.type(screen.getByLabelText("Descrição"), "Passagem");
    await user.type(screen.getByLabelText("Valor"), "800");
    await user.click(screen.getByRole("button", { name: "Adicionar transação hipotética" }));

    await waitFor(() =>
      expect(scenariosApi.createScenarioTransaction).toHaveBeenCalledWith(
        scenarioId,
        expect.objectContaining({ description: "Passagem", amount: -800 }),
      ),
    );
  });

  it("sends a positive amount when the transaction type is Receita", async () => {
    const user = userEvent.setup();
    vi.mocked(scenariosApi.listStandaloneScenarios).mockResolvedValue([scenario]);
    vi.mocked(scenariosApi.createScenarioTransaction).mockResolvedValue({
      id: "44444444-4444-4444-8444-444444444444",
      scenario_id: scenarioId,
      description: "Salário extra",
      amount: "500.00",
      projected_at: "2026-12-01",
      category: null,
      status: "projetada",
      realizations: [],
    });
    renderPage();
    await screen.findByText("Viagem");

    await user.type(screen.getByLabelText("Descrição"), "Salário extra");
    await user.click(screen.getByRole("combobox", { name: "Tipo" }));
    await user.click(await screen.findByText("Receita"));
    await user.type(screen.getByLabelText("Valor"), "500");
    await user.click(screen.getByRole("button", { name: "Adicionar transação hipotética" }));

    await waitFor(() =>
      expect(scenariosApi.createScenarioTransaction).toHaveBeenCalledWith(
        scenarioId,
        expect.objectContaining({ description: "Salário extra", amount: 500 }),
      ),
    );
  });
});
