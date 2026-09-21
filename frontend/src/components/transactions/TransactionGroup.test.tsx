import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  ignoredTransactionResult,
  transactionId,
  transactionResult,
} from "../../test/transactionFixtures";
import { TransactionGroup } from "./TransactionGroup";

describe("TransactionGroup", () => {
  it("shows full-period totals and split-page wording", () => {
    render(
      <TransactionGroup
        group={{
          ...transactionResult.groups[0]!,
          item_count: 3,
          page_item_count: 1,
          has_items_after: true,
        }}
        items={transactionResult.items}
      />,
    );
    expect(screen.getByRole("heading", { name: "1 de jul. – 31 de jul." })).toBeVisible();
    expect(screen.getByText(/continua em outra página/)).toBeVisible();
    expect(screen.getByText("Resultado")).toBeVisible();
    expect(screen.getAllByText(/-R\$\s*123,45/).length).toBeGreaterThan(0);
  });

  it("places null-dated transactions in a Sem data section", () => {
    const group = {
      ...transactionResult.groups[0]!,
      key: "undated",
      kind: "undated" as const,
      start_date: null,
      end_date: null,
    };
    render(
      <TransactionGroup
        group={group}
        items={[{ ...transactionResult.items[0]!, occurred_at: null, group_key: "undated" }]}
      />,
    );
    expect(screen.getByRole("heading", { name: "Sem data" })).toBeVisible();
  });

  it("opens details from the row and keeps Ignorar behind the row's menu", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const onInclusion = vi.fn();
    render(
      <TransactionGroup
        group={transactionResult.groups[0]!}
        items={transactionResult.items}
        onSelect={onSelect}
        onInclusion={onInclusion}
      />,
    );
    const details = screen.getByRole("button", { name: "Ver detalhes de Mercado" });
    const actions = screen.getByRole("button", { name: "Ações de Mercado" });
    expect(details).not.toContainElement(actions);
    expect(screen.queryByRole("menuitem", { name: "Ignorar" })).not.toBeInTheDocument();
    expect(screen.getByText("Conta corrente · Banco Teste")).toBeVisible();
    expect(screen.getAllByText("15 jul.").length).toBeGreaterThan(0);
    expect(screen.getAllByText(/123,45/).length).toBeGreaterThan(0);

    await user.click(actions);
    await user.click(await screen.findByRole("menuitem", { name: "Ignorar" }));
    expect(onInclusion).toHaveBeenCalledWith(transactionId, "ignored");
    expect(onSelect).not.toHaveBeenCalled();

    await user.click(details);
    expect(onSelect).toHaveBeenCalledWith(transactionId);
  });

  it("marks the row whose panel is open as current", () => {
    render(
      <TransactionGroup
        group={transactionResult.groups[0]!}
        items={transactionResult.items}
        selectedId={transactionId}
      />,
    );
    expect(screen.getByRole("button", { name: "Ver detalhes de Mercado" }).closest(".transaction-row")).toHaveAttribute(
      "aria-current",
      "true",
    );
  });

  it("shows the internal category name, falling back to Sem categoria", () => {
    render(
      <TransactionGroup group={transactionResult.groups[0]!} items={transactionResult.items} />,
    );
    expect(screen.getByText("Alimentação")).toBeVisible();

    render(
      <TransactionGroup
        group={transactionResult.groups[0]!}
        items={[{ ...transactionResult.items[0]!, internal_category: null }]}
      />,
    );
    expect(screen.getAllByText("Sem categoria").length).toBeGreaterThan(0);
  });

  it("keeps ignored data legible and offers restore on only the pending target", async () => {
    const user = userEvent.setup();
    const onInclusion = vi.fn();
    render(
      <TransactionGroup
        group={ignoredTransactionResult.groups[0]!}
        items={ignoredTransactionResult.items}
        onInclusion={onInclusion}
        pendingTransactionId={transactionId}
      />,
    );
    expect(screen.getByText("Ignorada")).toBeVisible();
    expect(screen.getByText("Mercado")).toBeVisible();
    const actions = screen.getByRole("button", { name: "Ações de Mercado" });
    expect(actions).toHaveClass("ant-btn-loading");
    await user.click(actions);
    expect(screen.queryByRole("menuitem", { name: "Restaurar" })).not.toBeInTheDocument();
    expect(onInclusion).not.toHaveBeenCalled();
  });
});
