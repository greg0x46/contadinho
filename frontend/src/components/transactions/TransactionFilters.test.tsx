import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { currentMonthFilters } from "../../hooks/useTransactions";
import { categoryFilterOptions } from "../../presentation/categoryLabels";
import { TransactionFilters } from "./TransactionFilters";

describe("TransactionFilters", () => {
  it("applies quick filters immediately and validates a custom period", async () => {
    const user = userEvent.setup();
    const onApply = vi.fn();
    render(
      <TransactionFilters
        applied={currentMonthFilters(new Date(2026, 6, 30))}
        facets={undefined}
        onApply={onApply}
        onClear={vi.fn()}
      />,
    );
    await user.type(screen.getByLabelText("Descrição"), "  Mercado  ");
    await waitFor(() =>
      expect(onApply).toHaveBeenLastCalledWith(
        expect.objectContaining({ description: "Mercado" }),
      ),
    );

    await user.click(screen.getByRole("combobox", { name: "Período" }));
    await user.click(await screen.findByText("Personalizado"));
    const start = screen.getByLabelText("Data inicial");
    const end = screen.getByLabelText("Data final");
    await user.clear(start);
    await user.type(start, "2026-08-10");
    await user.clear(end);
    await user.type(end, "2026-08-01");
    await user.click(screen.getByRole("button", { name: "Confirmar período" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("posterior");
    const callsAfterError = onApply.mock.calls.length;

    await user.clear(end);
    await user.type(end, "2026-08-31");
    await user.click(screen.getByRole("button", { name: "Confirmar período" }));
    expect(onApply.mock.calls.length).toBe(callsAfterError + 1);
    expect(onApply).toHaveBeenLastCalledWith(
      expect.objectContaining({ description: "Mercado", date_to: "2026-08-31" }),
    );
  }, 15_000);

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
    await user.click(screen.getByText("Saídas"));
    await user.click(screen.getByRole("button", { name: "Aplicar" }));
    expect(onApply).toHaveBeenLastCalledWith(
      expect.objectContaining({ classification: "outflow" }),
    );

    view.rerender(
      <TransactionFilters
        applied={{ ...applied, classification: "outflow" }}
        emptyValues={applied}
        facets={undefined}
        onApply={onApply}
        onClear={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "Filtros 1" })).toBeVisible();
    await user.click(screen.getByLabelText("Remover Saída"));
    expect(onApply).toHaveBeenLastCalledWith(
      expect.objectContaining({ classification: null }),
    );
  });

  it("validates value ranges numerically and accepts a single limit", async () => {
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
    await user.type(screen.getByLabelText("Valor mínimo"), "100");
    await user.type(screen.getByLabelText("Valor máximo"), "20");
    await user.click(screen.getByRole("button", { name: "Aplicar" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("mínimo");
    expect(onApply).not.toHaveBeenCalled();

    await user.clear(screen.getByLabelText("Valor máximo"));
    await user.click(screen.getByRole("button", { name: "Aplicar" }));
    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ amount_min: "100", amount_max: null }),
    );
  }, 30_000);

  it("filters by category_id when a category option is selected", async () => {
    const user = userEvent.setup();
    const applied = currentMonthFilters(new Date(2026, 6, 30));
    const onApply = vi.fn();
    render(
      <TransactionFilters
        applied={applied}
        emptyValues={applied}
        facets={{
          accounts: [],
          institutions: [],
          categories: [
            { id: "cat-expense", name: "Compras", kind: "expense", is_active: true, icon: "shopping", color: "#e64980" },
          ],
        }}
        onApply={onApply}
        onClear={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("combobox", { name: "Categoria" }));
    await user.click(await screen.findByText("Despesa: Compras"));
    await waitFor(() =>
      expect(onApply).toHaveBeenLastCalledWith(
        expect.objectContaining({ category_id: "cat-expense" }),
      ),
    );
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
