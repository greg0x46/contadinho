import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as transactionsApi from "../../api/transactions";
import type { Category } from "../../api/contracts";
import { QueryTestProvider } from "../../test/QueryTestProvider";
import { transactionResult } from "../../test/transactionFixtures";
import { TransactionDetailDrawer } from "./TransactionDetailDrawer";

vi.mock("../../api/transactions");

// The drawer reads a transaction's reconciliation, so it needs a query
// client. These cases are about the other sections; the reconciliation
// section has its own file.
function renderWithRouter(ui: ReactElement) {
  return render(
    <MemoryRouter>
      <QueryTestProvider>{ui}</QueryTestProvider>
    </MemoryRouter>,
  );
}

const activeCategories: Category[] = [
  {
    id: "33333333-3333-4333-8333-333333333333",
    name: "Alimentação",
    kind: "expense",
    is_active: true,
    icon: "coffee",
    color: "#eb6834",
    created_at: "2026-07-31T00:00:00Z",
    updated_at: "2026-07-31T00:00:00Z",
  },
  {
    id: "44444444-4444-4444-8444-444444444444",
    name: "Transporte",
    kind: "expense",
    is_active: true,
    icon: "car",
    color: "#17a2b8",
    created_at: "2026-07-31T00:00:00Z",
    updated_at: "2026-07-31T00:00:00Z",
  },
];

describe("TransactionDetailDrawer", () => {
  beforeEach(() => {
    vi.mocked(transactionsApi.getTransactionReconciliation).mockResolvedValue({
      current: null,
      options: [],
    });
  });

  it("assigns a category manually and shows the provider suggestion separately", async () => {
    const user = userEvent.setup();
    const onCategory = vi.fn();
    renderWithRouter(
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
    renderWithRouter(
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
    renderWithRouter(
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

  it("renders the transfer-category exclusion reason, with the transaction still considered", async () => {
    const user = userEvent.setup();
    renderWithRouter(
      <TransactionDetailDrawer
        item={{
          ...transactionResult.items[0]!,
          inclusion: { state: "considered", changed_at: null, origin: "manual", rule_name: null },
          totals_eligibility: { included: false, reason: "transfer_category" },
        }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    await user.click(screen.getByText("Informações técnicas"));
    expect(
      await screen.findByText("Fora dos totais: transferência entre contas próprias"),
    ).toBeVisible();
  });

  it("shows Sem categoria as a placeholder when no internal category is assigned", () => {
    renderWithRouter(
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
    renderWithRouter(
      <TransactionDetailDrawer
        item={{ ...transactionResult.items[0]!, card: null }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.queryByText("Cartão")).not.toBeInTheDocument();
  });

  it("shows the masked card number when present", () => {
    renderWithRouter(
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

  it("does not show edit/delete actions for a synced transaction", () => {
    renderWithRouter(
      <TransactionDetailDrawer
        item={{ ...transactionResult.items[0]!, origin: "synced" }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.queryByText("Manual")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Editar" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Excluir" })).not.toBeInTheDocument();
  });

  it("shows the Manual tag and edit/delete actions for a manual transaction", async () => {
    const user = userEvent.setup();
    const onEditManual = vi.fn();
    const onDeleteManual = vi.fn();
    renderWithRouter(
      <TransactionDetailDrawer
        item={{ ...transactionResult.items[0]!, origin: "manual" }}
        categories={activeCategories}
        onClose={vi.fn()}
        onEditManual={onEditManual}
        onDeleteManual={onDeleteManual}
      />,
    );
    expect(screen.getByText("Manual")).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Editar" }));
    expect(onEditManual).toHaveBeenCalledWith({ ...transactionResult.items[0]!, origin: "manual" });

    const deleteTrigger = screen.getByRole("button", { name: "Excluir" });
    await user.click(deleteTrigger);
    const buttons = await screen.findAllByRole("button", { name: "Excluir" });
    const confirmButton = buttons.find((button) => button !== deleteTrigger)!;
    await user.click(confirmButton);
    expect(onDeleteManual).toHaveBeenCalledWith(transactionResult.items[0]!.id);
  });

  it("shows the delete error when a manual delete is rejected", () => {
    renderWithRouter(
      <TransactionDetailDrawer
        item={{ ...transactionResult.items[0]!, origin: "manual" }}
        categories={activeCategories}
        onClose={vi.fn()}
        onDeleteManual={vi.fn()}
        deleteManualError="Não foi possível excluir o lançamento."
      />,
    );
    expect(screen.getByText("Não foi possível excluir o lançamento.")).toBeVisible();
  });

  it("shows the installment badge when the purchase is parcelada", () => {
    renderWithRouter(
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

  it("describes the remainder of a linked aporte as spending", () => {
    renderWithRouter(
      <TransactionDetailDrawer
        item={{
          ...transactionResult.items[0]!,
          investment_transfer_amount: "100.0000",
          reportable_amount: "23.4500",
        }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(
      screen.getByText(/aporte vinculado: R\$\s100,00 · restante nos gastos: R\$\s23,45/),
    ).toBeVisible();
  });

  it("describes the remainder of a linked resgate as income", () => {
    renderWithRouter(
      <TransactionDetailDrawer
        item={{
          ...transactionResult.items[0]!,
          classification: "inflow",
          movement_type: "CREDIT",
          investment_transfer_amount: "100.0000",
          reportable_amount: "23.4500",
        }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(
      screen.getByText(/resgate vinculado: R\$\s100,00 · restante nas receitas: R\$\s23,45/),
    ).toBeVisible();
    expect(screen.queryByText(/restante nos gastos/)).toBeNull();
  });

  it("says an ignored linked line is out of the totals instead of showing a zero remainder", () => {
    renderWithRouter(
      <TransactionDetailDrawer
        item={{
          ...transactionResult.items[0]!,
          investment_transfer_amount: "100.0000",
          reportable_amount: "0",
          inclusion: { state: "ignored", changed_at: null, origin: "manual", rule_name: null },
          totals_eligibility: { included: false, reason: "ignored" },
        }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText(/aporte vinculado: R\$\s100,00 · fora dos totais/)).toBeVisible();
    expect(screen.queryByText(/restante/)).toBeNull();
  });
});
