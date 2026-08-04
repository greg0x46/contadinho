import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { Category } from "../../api/contracts";
import { transactionResult } from "../../test/transactionFixtures";
import { TransactionDetailDrawer } from "./TransactionDetailDrawer";

const activeCategories: Category[] = [
  {
    id: "33333333-3333-4333-8333-333333333333",
    name: "Alimentação",
    kind: "expense",
    is_active: true,
    created_at: "2026-07-31T00:00:00Z",
    updated_at: "2026-07-31T00:00:00Z",
  },
  {
    id: "44444444-4444-4444-8444-444444444444",
    name: "Transporte",
    kind: "expense",
    is_active: true,
    created_at: "2026-07-31T00:00:00Z",
    updated_at: "2026-07-31T00:00:00Z",
  },
];

describe("TransactionDetailDrawer", () => {
  it("assigns a category manually and shows the provider suggestion separately", async () => {
    const user = userEvent.setup();
    const onCategory = vi.fn();
    render(
      <TransactionDetailDrawer
        item={transactionResult.items[0]!}
        categories={activeCategories}
        onClose={vi.fn()}
        onCategory={onCategory}
      />,
    );

    expect(screen.getByText("Alimentação")).toBeVisible();
    expect(screen.getByText(/não afeta filtros ou totais/)).toBeVisible();

    await user.click(screen.getByRole("combobox", { name: "Categoria" }));
    await user.click(await screen.findByText("Despesa: Transporte"));

    expect(onCategory).toHaveBeenCalledWith(transactionResult.items[0]!.id, "44444444-4444-4444-8444-444444444444");
  });

  it("warns when the vigente category is inactive without blocking display", () => {
    render(
      <TransactionDetailDrawer
        item={{
          ...transactionResult.items[0]!,
          internal_category: {
            ...transactionResult.items[0]!.internal_category!,
            is_active: false,
          },
        }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText(/continua vigente para esta transação/)).toBeVisible();
  });

  it("renders the unclassified totalization exclusion reason", async () => {
    const user = userEvent.setup();
    render(
      <TransactionDetailDrawer
        item={{
          ...transactionResult.items[0]!,
          totals_eligibility: { included: false, reason: "unclassified" },
        }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    await user.click(screen.getByText("Informações técnicas"));
    expect(await screen.findByText("Fora dos totais: tipo não classificado")).toBeVisible();
  });

  it("shows Sem categoria as a placeholder when no internal category is assigned", () => {
    render(
      <TransactionDetailDrawer
        item={{ ...transactionResult.items[0]!, internal_category: null }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByRole("combobox", { name: "Categoria" })).toBeVisible();
    expect(screen.getByText("Sem categoria")).toBeVisible();
  });

  it("does not show a Cartão row when the transaction has no card info", () => {
    render(
      <TransactionDetailDrawer
        item={{ ...transactionResult.items[0]!, card: null }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.queryByText("Cartão")).not.toBeInTheDocument();
  });

  it("shows the masked card number when present", () => {
    render(
      <TransactionDetailDrawer
        item={{
          ...transactionResult.items[0]!,
          card: { number: "**** 4321", installment_number: null, total_installments: null },
        }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText("Cartão")).toBeVisible();
    expect(screen.getByText("**** 4321")).toBeVisible();
    expect(screen.queryByText(/Parcela/)).not.toBeInTheDocument();
  });

  it("shows the installment badge when the purchase is parcelada", () => {
    render(
      <TransactionDetailDrawer
        item={{
          ...transactionResult.items[0]!,
          card: { number: "**** 1234", installment_number: 3, total_installments: 12 },
        }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText("**** 1234")).toBeVisible();
    expect(screen.getByText("Parcela 3/12")).toBeVisible();
  });
});
