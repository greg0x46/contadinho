import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as automationRulesApi from "../api/automationRules";
import type { AutomationRule } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { AutomationRulesPage } from "./AutomationRulesPage";

vi.mock("../api/automationRules");

const ruleId = "33333333-3333-4333-8333-333333333333";

const rule: AutomationRule = {
  id: ruleId,
  name: "Ignorar taxas",
  is_active: true,
  logic_operator: "or",
  conditions: [{ field: "description", operator: "contains", value: "taxa" }],
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
  });

  it("lists existing rules with a readable condition summary", async () => {
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([rule]);
    renderPage();
    expect(await screen.findByText("Ignorar taxas")).toBeVisible();
    expect(screen.getByText('Descrição contém "taxa"')).toBeVisible();
  });

  it("blocks saving a new rule without a name or with an empty condition value", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([]);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova regra" }));

    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um nome para a regra.")).toBeVisible();
    expect(automationRulesApi.createAutomationRule).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText("Nome"), "Nova regra de teste");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Preencha o valor de todas as condições.")).toBeVisible();
    expect(automationRulesApi.createAutomationRule).not.toHaveBeenCalled();
  });

  it("creates a rule with the entered name and condition", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([]);
    vi.mocked(automationRulesApi.createAutomationRule).mockResolvedValue({
      rule,
      retroactive_apply: null,
    });
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova regra" }));
    await user.type(screen.getByLabelText("Nome"), rule.name);
    await user.type(screen.getByLabelText("Valor"), "taxa");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(automationRulesApi.createAutomationRule).toHaveBeenCalledWith({
        name: rule.name,
        is_active: true,
        logic_operator: "or",
        conditions: [{ field: "description", operator: "contains", value: "taxa" }],
        apply_retroactively: false,
      }),
    );
    expect(screen.queryByRole("button", { name: "Salvar" })).not.toBeInTheDocument();
  });

  it("lists existing accounts and cards in selects", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([]);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova regra" }));

    await user.click(screen.getByRole("combobox", { name: "Campo" }));
    await user.click(await screen.findByText("Conta"));
    await user.click(screen.getByRole("combobox", { name: "Valor" }));
    const accountOption = await screen.findByRole(
      "option",
      { name: "ultraviolet-black" },
      { timeout: 5000 },
    );
    expect(accountOption).toBeInTheDocument();
    await user.click(accountOption);

    await user.click(screen.getByRole("combobox", { name: "Campo" }));
    await user.click(await screen.findByText("Cartão"));
    await user.click(screen.getByRole("combobox", { name: "Valor" }));
    expect(
      await screen.findByRole("option", { name: "1139" }, { timeout: 5000 }),
    ).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "2848" })).toBeInTheDocument();
  });

  it("shows the retroactive apply result after saving with the checkbox marked", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([]);
    vi.mocked(automationRulesApi.createAutomationRule).mockResolvedValue({
      rule,
      retroactive_apply: { matched: 3, ignored: 2 },
    });
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova regra" }));
    await user.type(screen.getByLabelText("Nome"), rule.name);
    await user.type(screen.getByLabelText("Valor"), "taxa");
    await user.click(
      screen.getByText("Aplicar agora às transações existentes que casarem com as condições"),
    );
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(automationRulesApi.createAutomationRule).toHaveBeenCalledWith(
        expect.objectContaining({ apply_retroactively: true }),
      ),
    );
    expect(
      await screen.findByText("3 transação(ões) encontrada(s), 2 ignorada(s) agora."),
    ).toBeVisible();
  });

  it("edits an existing rule pre-filled with its current values", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([rule]);
    vi.mocked(automationRulesApi.updateAutomationRule).mockResolvedValue({
      rule: { ...rule, name: "Renomeada" },
      retroactive_apply: null,
    });
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Editar" }));
    const nameField = await screen.findByLabelText("Nome");
    expect(nameField).toHaveValue(rule.name);
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

  it("deletes a rule after confirmation", async () => {
    const user = userEvent.setup();
    vi.mocked(automationRulesApi.listAutomationRules).mockResolvedValue([rule]);
    vi.mocked(automationRulesApi.deleteAutomationRule).mockResolvedValue(undefined);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Excluir" }));
    const confirm = await screen.findByRole("tooltip");
    await user.click(within(confirm).getByRole("button", { name: "Excluir" }));

    await waitFor(() =>
      expect(automationRulesApi.deleteAutomationRule).toHaveBeenCalledWith(ruleId),
    );
  });
});
