import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SyncFailureList } from "./SyncFailureList";

describe("SyncFailureList", () => {
  it("renders sanitized messages as text and optional contexts conditionally", () => {
    const message = '<img src=x onerror="window.hacked=true">';
    render(
      <SyncFailureList
        failures={[
          {
            stage: "transactions",
            code: "safe",
            message,
            external_account_id: "account-long-id",
            external_transaction_id: null,
            occurred_at: "2026-07-29T12:00:00Z",
          },
        ]}
      />,
    );
    expect(screen.getByText(message)).toBeVisible();
    expect(document.querySelector("img")).toBeNull();
    expect(screen.getByText("account-long-id")).toBeVisible();
    expect(screen.queryByText("Transação relacionada")).toBeNull();
  });
});
