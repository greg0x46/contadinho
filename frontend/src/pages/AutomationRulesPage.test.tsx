import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as automationRulesApi from "../api/automationRules";
import * as categoriesApi from "../api/categories";
import * as recurringCommitmentsApi from "../api/recurringCommitments";
import type { AutomationRule, Category, RecurringCommitment } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { AutomationRulesPage } from "./AutomationRulesPage";

vi.mock("../api/automationRules");
vi.mock("../api/recurringCommitments");
vi.mock("../api/categories");

const ruleId = "33333333-3333-4333-8333-333333333333";
const reconcileRuleId = "66666666-6666-4666-8666-666666666666";
const commitmentId = "55555555-5555-4555-8555-555555555555";
const categoryId = "44444444-4444-4444-8444-444444444444";

const rule: AutomationRule = {
  id: ruleId,
  name: "Ignorar taxas",
  is_active: true,
  logic_operator: "or",
  conditions: [{ field: "description", operator: "contains", value: "taxa" }],
  actions: [{ type: "ignore", recurring_commitment_id: null }],
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

const category: Category = {
  id: categoryId,
  name: "Moradia",
  kind: "expense",
  is_active: true,
  icon: "home",
  color: "#495057",
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

const commitment: RecurringCommitment = {
  id: commitmentId,
  name: "Aluguel",
  kind: "expense",
  amount: "1500.00",
  category_id: categoryId,
  account_id: null,
  cadence: "monthly",
  day_of_month: 5,
  month_of_year: null,
  start_date: "2026-01-01",
  end_date: null,
  is_active: true,
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

const reconcileRule: AutomationRule = {
  id: reconcileRuleId,
  name: "Concilia aluguel",
  is_active: true,
  logic_operator: "and",
  conditions: [{ field: "amount", operator: "within_percent", value: "10" }],
  actions: [{ type: "reconcile", recurring_commitment_id: commitmentId }],
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <AutomationRulesPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("AutomationRulesPage", () => {
  beforeEach(() => {
    vi.mocked(automationRulesApi.listAutomationRuleConditionOptions).mockResolvedValue({
      accounts: ["ultraviolet-black"],
      cards: ["1139", "2848"],
    });
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([category]);
    vi.mocked(recurringCommitmentsApi.listRecurringCommitments).mockResolvedValue([]);
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([]);
  });

  it("lists ignore and reconcile rules, resolving the reconcile row's linked commitment name", async () => {
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([rule, reconcileRule]);
    vi.mocked(recurringCommitmentsApi.listRecurringCommitments).mockResolvedValue([commitment]);
    renderPage();

    expect(await screen.findByText("Ignorar taxas")).toBeVisible();
    expect(screen.getByText("Concilia aluguel")).toBeVisible();
    expect(screen.getByText("Ignorar")).toBeVisible();
    expect(screen.getByText("Concilia: Aluguel")).toBeVisible();
  });

  it("creates an ignore rule with the entered name and condition", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.createAutomationRule).mockResolvedValue({
      rule,
      retroactive_apply: null,
    });
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova automação" }));
    await user.type(screen.getByLabelText("Nome da automação"), rule.name);
    await user.type(screen.getByLabelText("Valor"), "taxa");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(automationRulesApi.createAutomationRule).toHaveBeenCalledWith({
        name: rule.name,
        is_active: true,
        logic_operator: "or",
        conditions: [{ field: "description", operator: "contains", value: "taxa" }],
        actions: [{ type: "ignore", recurring_commitment_id: null }],
        apply_retroactively: false,
      }),
    );
    expect(screen.queryByRole("button", { name: "Salvar" })).not.toBeInTheDocument();
  });

  it("creates a reconciliation automation: creates the commitment, then the rule referencing it", async () => {
    const user = userEvent.setup();
    vi.mocked(recurringCommitmentsApi.createRecurringCommitment).mockResolvedValue(commitment);
    vi.mocked(automationRulesApi.createAutomationRule).mockResolvedValue({
      rule: reconcileRule,
      retroactive_apply: null,
    });
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova automação" }));
    await user.type(screen.getByLabelText("Nome da automação"), "Concilia aluguel");
    await user.click(screen.getByText("Conciliar recorrência"));

    await user.type(screen.getByLabelText("Nome"), "Aluguel");
    await user.type(screen.getByLabelText("Valor"), "1500");
    await user.click(screen.getByRole("combobox", { name: "Categoria" }));
    await user.click(await screen.findByText("Moradia"));
    await user.type(screen.getByLabelText("Dia do mês"), "5");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(recurringCommitmentsApi.createRecurringCommitment).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Aluguel",
          amount: "1500.00",
          category_id: categoryId,
          day_of_month: 5,
          cadence: "monthly",
        }),
      ),
    );
    await waitFor(() =>
      expect(automationRulesApi.createAutomationRule).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Concilia aluguel",
          actions: [{ type: "reconcile", recurring_commitment_id: commitmentId }],
        }),
      ),
    );
  });

  it("edits an existing rule pre-filled with its current values, with the action locked", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([rule]);
    vi.mocked(automationRulesApi.updateAutomationRule).mockResolvedValue({
      rule: { ...rule, name: "Renomeada" },
      retroactive_apply: null,
    });
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Editar" }));
    const nameField = await screen.findByLabelText("Nome da automação");
    expect(nameField).toHaveValue(rule.name);
    expect(screen.getByRole("radio", { name: "Ignorar transação" })).toBeDisabled();
    await user.clear(nameField);
    await user.type(nameField, "Renomeada");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(automationRulesApi.updateAutomationRule).toHaveBeenCalledWith(
        ruleId,
        expect.objectContaining({ name: "Renomeada" }),
      ),
    );
  });

  it("editing a reconcile rule pre-fills the linked commitment's fields", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([reconcileRule]);
    vi.mocked(recurringCommitmentsApi.listRecurringCommitments).mockResolvedValue([commitment]);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Editar" }));

    expect(await screen.findByLabelText("Nome da automação")).toHaveValue(reconcileRule.name);
    expect(screen.getByLabelText("Nome")).toHaveValue(commitment.name);
  });

  it("toggles a rule active state", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([rule]);
    vi.mocked(automationRulesApi.setAutomationRuleActive).mockResolvedValue({
      ...rule,
      is_active: false,
    });
    renderPage();
    const toggle = await screen.findByRole("switch");
    await user.click(toggle);

    await waitFor(() =>
      expect(automationRulesApi.setAutomationRuleActive).toHaveBeenCalledWith(ruleId, false),
    );
  });

  it("deletes a rule after confirmation, leaving its linked commitment intact", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([reconcileRule]);
    vi.mocked(recurringCommitmentsApi.listRecurringCommitments).mockResolvedValue([commitment]);
    vi.mocked(automationRulesApi.deleteAutomationRule).mockResolvedValue(undefined);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Excluir" }));
    const confirm = await screen.findByRole("tooltip");
    await user.click(within(confirm).getByRole("button", { name: "Excluir" }));

    await waitFor(() =>
      expect(automationRulesApi.deleteAutomationRule).toHaveBeenCalledWith(reconcileRuleId),
    );
    expect(recurringCommitmentsApi.deleteRecurringCommitment).not.toHaveBeenCalled();
  });
});
