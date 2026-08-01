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
    expect(screen.getByText("Saldo")).toBeVisible();
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

  it("renders sibling detail and explicit ignore controls without hiding facts", async () => {
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
    const ignore = screen.getByRole("button", { name: "Ignorar Mercado" });
    expect(details).not.toContainElement(ignore);
    expect(screen.getByText("Conta corrente · Banco Teste")).toBeVisible();
    expect(screen.getAllByText(/123,45/).length).toBeGreaterThan(0);
    await user.click(ignore);
    expect(onInclusion).toHaveBeenCalledWith(transactionId, "ignored");
    await user.click(details);
    expect(onSelect).toHaveBeenCalledWith(transactionId);
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
    const restore = screen.getByRole("button", { name: "Restaurar Mercado" });
    expect(restore).toHaveClass("ant-btn-loading");
    await user.click(restore);
    expect(onInclusion).not.toHaveBeenCalled();
  });
});
