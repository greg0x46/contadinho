import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { transactionResult } from "../../test/transactionFixtures";
import { TransactionRow } from "./TransactionRow";

const item = transactionResult.items[0]!;

function renderRow(layout: "phone" | "narrow" | "wide", overrides = {}) {
  return render(
    <TransactionRow item={{ ...item, ...overrides }} layout={layout} onSelect={() => undefined} />,
  ).container;
}

describe("TransactionRow layout", () => {
  it("wide: the category is its own column, not part of the meta line", () => {
    const row = renderRow("wide");
    expect(row.querySelector(".transaction-row > .transaction-category")).not.toBeNull();
    expect(row.querySelector(".transaction-meta .transaction-category")).toBeNull();
    expect(row.querySelector(".transaction-row.is-folded")).toBeNull();
  });

  it("narrow: no category column; it joins the meta line and the row is marked folded", () => {
    const row = renderRow("narrow");
    expect(row.querySelector(".transaction-row > .transaction-category")).toBeNull();
    expect(row.querySelector(".transaction-meta .transaction-category")).not.toBeNull();
    expect(row.querySelector(".transaction-row.is-folded")).not.toBeNull();
    // The date is still a column at this width.
    expect(row.querySelector(".transaction-meta-date")).toBeNull();
  });

  it("phone: the date and category fold into the meta line, and the account yields to the category", () => {
    const row = renderRow("phone");
    expect(row.querySelector(".transaction-meta-date")).not.toBeNull();
    expect(row.querySelector(".transaction-meta .transaction-category")).not.toBeNull();
    expect(row.querySelector(".transaction-meta-where")).toBeNull();
  });

  it("phone: without a category the account stays in the meta line", () => {
    const row = renderRow("phone", { internal_category: null });
    expect(row.querySelector(".transaction-meta-where")).not.toBeNull();
  });

  it("carries the transaction id on the opening button, for returning focus to it", () => {
    const row = renderRow("wide");
    expect(row.querySelector(".transaction-row-open")?.getAttribute("data-transaction-id")).toBe(item.id);
  });
});
