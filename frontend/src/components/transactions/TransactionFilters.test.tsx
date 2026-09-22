import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { currentMonthFilters } from "../../hooks/useTransactions";
import { categoryFilterOptions } from "../../presentation/categoryLabels";
import { TransactionFilters } from "./TransactionFilters";

describe("TransactionFilters", () => {
  it("applies the search immediately, trimmed, without touching the period", async () => {
    const user = userEvent.setup();
    const onApply = vi.fn();
    render(
      <TransactionFilters
        applied={{ ...currentMonthFilters(new Date(2026, 6, 30)), category_ids: ["category"] }}
        facets={undefined}
        onApply={onApply}
        onClear={vi.fn()}
      />,
    );
    // The period is page context, not a collection control: nothing here
    // navigates it.
    expect(screen.queryByRole("button", { name: "Selecionar período" })).not.toBeInTheDocument();
    await user.type(screen.getByLabelText("Descrição"), "  Mercado  ");
    await waitFor(() =>
      expect(onApply).toHaveBeenLastCalledWith(
        expect.objectContaining({
          description: "Mercado",
          category_ids: ["category"],
          date_from: "2026-07-01",
          date_to: "2026-07-31",
        }),
      ),
    );
  });

  it("renders the caller's shaping controls beside the filters", () => {
    render(
      <TransactionFilters
        applied={currentMonthFilters(new Date(2026, 6, 30))}
        facets={undefined}
        onApply={vi.fn()}
        onClear={vi.fn()}
        end={<button type="button">Agrupar</button>}
      />,
    );
    const toolbar = screen.getByRole("region", { name: "Filtros de transações" });
    expect(toolbar).toContainElement(screen.getByRole("button", { name: "Agrupar" }));
    expect(toolbar).toContainElement(screen.getByRole("button", { name: "Filtros" }));
  });

  it("counts, applies and removes advanced filters", async () => {
    const user = userEvent.setup();
    const applied = currentMonthFilters(new Date(2026, 6, 30));
    const onApply = vi.fn();
    const view = render(
      <TransactionFilters
        applied={applied}
        emptyValues={applied}
        facets={undefined}
        onApply={onApply}
        onClear={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Filtros" }));
    await user.click(screen.getByRole("checkbox", { name: "Sem categoria" }));
    await user.click(screen.getByRole("button", { name: "Aplicar" }));
    expect(onApply).toHaveBeenLastCalledWith(
      expect.objectContaining({ uncategorized: true }),
    );

    view.rerender(
      <TransactionFilters
        applied={{ ...applied, uncategorized: true }}
        emptyValues={applied}
        facets={undefined}
        onApply={onApply}
        onClear={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "Filtros 1" })).toBeVisible();
    await user.click(screen.getByLabelText("Remover Sem categoria"));
    expect(onApply).toHaveBeenLastCalledWith(
      expect.objectContaining({ uncategorized: null }),
    );
  });

  it("validates value ranges numerically and accepts a single limit", async () => {
    // The money inputs are a cents mask: "10000" reads as R$ 100,00.
    const user = userEvent.setup();
    const applied = currentMonthFilters(new Date(2026, 6, 30));
    const onApply = vi.fn();
    render(
      <TransactionFilters
        applied={applied}
        emptyValues={applied}
        facets={undefined}
        onApply={onApply}
        onClear={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Filtros" }));
    await user.type(screen.getByLabelText("Valor mínimo"), "10000");
    expect(screen.getByLabelText("Valor mínimo")).toHaveValue("R$\u00a0100,00");
    await user.type(screen.getByLabelText("Valor máximo"), "2000");
    await user.click(screen.getByRole("button", { name: "Aplicar" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("mínimo");
    expect(onApply).not.toHaveBeenCalled();

    await user.clear(screen.getByLabelText("Valor máximo"));
    await user.click(screen.getByRole("button", { name: "Aplicar" }));
    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ amount_min: "100.00", amount_max: null }),
    );
  }, 30_000);

  it("filters by several categories at once, and the panel counts them as one filter", async () => {
    const user = userEvent.setup();
    const applied = currentMonthFilters(new Date(2026, 6, 30));
    const onApply = vi.fn();
    const view = render(
      <TransactionFilters
        applied={applied}
        emptyValues={applied}
        facets={{
          accounts: [],
          institutions: [],
          categories: [
            { id: "cat-expense", name: "Compras", kind: "expense", is_active: true, icon: "shopping", color: "#e64980" },
            { id: "cat-food", name: "Alimentação", kind: "expense", is_active: true, icon: "coffee", color: "#e67e22" },
          ],
        }}
        onApply={onApply}
        onClear={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Filtros" }));
    await user.click(await screen.findByRole("button", { name: "Categoria" }));
    // Picking does not close the list: both go in before leaving.
    await user.click(await screen.findByRole("option", { name: "Despesa: Compras" }));
    await user.click(screen.getByRole("option", { name: "Despesa: Alimentação" }));
    expect(screen.getByRole("option", { name: "Despesa: Compras" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("2 categorias selecionadas")).toBeVisible();
    await user.keyboard("{Escape}");
    expect(screen.getByRole("button", { name: "Categoria" })).toHaveTextContent("Despesa: Compras +1");
    await user.click(screen.getByRole("button", { name: "Aplicar" }));
    await waitFor(() =>
      expect(onApply).toHaveBeenLastCalledWith(
        expect.objectContaining({ category_ids: ["cat-expense", "cat-food"] }),
      ),
    );

    view.rerender(
      <TransactionFilters
        applied={{ ...applied, category_ids: ["cat-expense", "cat-food"], classification: "inflow" }}
        emptyValues={applied}
        facets={{ accounts: [], institutions: [], categories: [] }}
        onApply={onApply}
        onClear={vi.fn()}
      />,
    );
    // Two categories and a movement are two active filters, not three.
    expect(screen.getByRole("button", { name: "Filtros 2" })).toBeVisible();
  });

  it("navigates the account list by keyboard and summarizes long selections", async () => {
    const user = userEvent.setup();
    const applied = currentMonthFilters(new Date(2026, 6, 30));
    const accounts = ["Nubank", "Itaú", "Inter", "XP"].map((name, index) => ({
      id: `acc-${index}`,
      name,
      institution: null,
    }));
    render(
      <TransactionFilters
        applied={applied}
        emptyValues={applied}
        facets={{ accounts, institutions: [], categories: [] }}
        onApply={vi.fn()}
        onClear={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Filtros" }));
    await user.click(await screen.findByRole("button", { name: "Conta" }));
    const search = await screen.findByRole("combobox", { name: "Buscar em Contas" });
    await waitFor(() => expect(search).toHaveFocus());
    await user.keyboard("{Enter}{ArrowDown}{Enter}{ArrowDown}{Enter}");
    expect(screen.getByText("3 contas selecionadas")).toBeVisible();
    await user.type(search, "xp");
    expect(screen.getAllByRole("option")).toHaveLength(1);
    await user.keyboard("{Enter}");
    await user.clear(search);
    expect(screen.getByRole("option", { name: "Nubank" })).toHaveAttribute("aria-selected", "true");
    await user.keyboard("{Escape}");
    const trigger = screen.getByRole("button", { name: "Conta" });
    expect(trigger).toHaveFocus();
    expect(trigger).toHaveTextContent("Nubank +3");
  });

  it("groups category options by kind, sorting inactive categories last within their kind", () => {
    const options = categoryFilterOptions([
      { id: "income", name: "Salário", kind: "income", is_active: true, icon: "money-collect", color: "#1baf7a" },
      { id: "inactive", name: "Velha", kind: "expense", is_active: false, icon: "ellipsis", color: "#6c757d" },
      { id: "expense", name: "Compras", kind: "expense", is_active: true, icon: "shopping", color: "#e64980" },
      { id: "transfer", name: "Entre Contas", kind: "transfer", is_active: true, icon: "swap", color: "#7c5cbf" },
    ]);
    expect(options).toEqual([
      { value: "expense", label: "Despesa: Compras", icon: "shopping", color: "#e64980" },
      { value: "inactive", label: "Despesa: Velha (inativa)", icon: "ellipsis", color: "#6c757d" },
      { value: "income", label: "Receita: Salário", icon: "money-collect", color: "#1baf7a" },
      { value: "transfer", label: "Transferência: Entre Contas", icon: "swap", color: "#7c5cbf" },
    ]);
  });
});
