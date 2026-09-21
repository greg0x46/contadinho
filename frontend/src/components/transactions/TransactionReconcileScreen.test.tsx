import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as recurringCommitmentsApi from "../../api/recurringCommitments";
import * as transactionsApi from "../../api/transactions";
import type { TransactionReconciliation } from "../../api/contracts";
import { QueryTestProvider } from "../../test/QueryTestProvider";
import { transactionResult } from "../../test/transactionFixtures";
import { TransactionReconcileScreen } from "./TransactionReconcileScreen";

vi.mock("../../api/transactions");
vi.mock("../../api/recurringCommitments");

const transaction = transactionResult.items[0]!;
const commitmentId = "22222222-2222-4222-8222-222222222222";

const unreconciled: TransactionReconciliation = {
  current: null,
  options: [
    {
      commitment_id: commitmentId,
      commitment_name: "Aluguel",
      kind: "expense",
      occurrence_date: "2026-02-05",
      expected_amount: "1500.00",
    },
  ],
};

// antd's Popconfirm gives its confirm button the same label as the trigger
// that opened it, so the click has to be scoped to the popover itself.
async function confirmPopconfirm(user: ReturnType<typeof userEvent.setup>, label: string) {
  const popover = await waitFor(() => {
    const found = document.querySelector<HTMLElement>(".ant-popconfirm");
    if (found === null) throw new Error("popconfirm not open");
    return found;
  });
  await user.click(within(popover).getByRole("button", { name: label }));
}

function renderScreen() {
  const onDone = vi.fn();
  render(
    <QueryTestProvider>
      <TransactionReconcileScreen item={transaction} onDone={onDone} />
    </QueryTestProvider>,
  );
  return { onDone };
}

describe("TransactionReconcileScreen", () => {
  beforeEach(() => {
    vi.mocked(recurringCommitmentsApi.putReconciliation).mockResolvedValue({
      date: "2026-02-05",
      expected_amount: "1500.00",
      status: "reconciled",
      origin: "manual",
      transaction: null,
    });
  });

  it("reconciles a transaction with a chosen occurrence on Conciliar and hands back", async () => {
    const user = userEvent.setup();
    vi.mocked(transactionsApi.getTransactionReconciliation).mockResolvedValue(unreconciled);
    const { onDone } = renderScreen();

    await user.click(await screen.findByRole("combobox", { name: "Conciliar com uma recorrência" }));
    // The row splits description, date and amount across elements.
    const description = await screen.findByText("Aluguel");
    expect(description.closest(".ant-select-item-option")).toHaveTextContent("05 fev.");
    await user.click(description);
    expect(recurringCommitmentsApi.putReconciliation).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Conciliar" }));
    await waitFor(() =>
      expect(recurringCommitmentsApi.putReconciliation).toHaveBeenCalledWith(commitmentId, "2026-02-05", {
        state: "linked",
        transaction_id: transaction.id,
      }),
    );
    await waitFor(() => expect(onDone).toHaveBeenCalled());
  });

  it("asks for an occurrence before writing", async () => {
    const user = userEvent.setup();
    vi.mocked(transactionsApi.getTransactionReconciliation).mockResolvedValue(unreconciled);
    renderScreen();

    await user.click(await screen.findByRole("button", { name: "Conciliar" }));
    expect(await screen.findByText("Selecione uma ocorrência para conciliar.")).toBeVisible();
    expect(recurringCommitmentsApi.putReconciliation).not.toHaveBeenCalled();
  });

  it("explains when no compatible occurrence is nearby instead of offering an empty picker", async () => {
    vi.mocked(transactionsApi.getTransactionReconciliation).mockResolvedValue({ current: null, options: [] });
    renderScreen();

    expect(await screen.findByText(/Nenhuma ocorrência de recorrência compatível/)).toBeVisible();
    expect(screen.getByRole("button", { name: "Conciliar" })).toBeDisabled();
  });

  it("shows the current reconciliation with its origin", async () => {
    vi.mocked(transactionsApi.getTransactionReconciliation).mockResolvedValue({
      current: {
        commitment_id: commitmentId,
        commitment_name: "Aluguel",
        kind: "expense",
        occurrence_date: "2026-02-05",
        expected_amount: "1500.00",
        origin: "rule",
      },
      options: [],
    });
    renderScreen();

    expect(await screen.findByText("Aluguel")).toBeVisible();
    expect(screen.getByText("Conciliação automática")).toBeVisible();
  });

  // Desconciliar must persist a decision, not just delete a link: the
  // automation rule is re-evaluated on every read and would otherwise
  // re-claim the occurrence immediately.
  it("desconciliar writes a detached decision, even for a rule-driven match", async () => {
    const user = userEvent.setup();
    vi.mocked(transactionsApi.getTransactionReconciliation).mockResolvedValue({
      current: {
        commitment_id: commitmentId,
        commitment_name: "Aluguel",
        kind: "expense",
        occurrence_date: "2026-02-05",
        expected_amount: "1500.00",
        origin: "rule",
      },
      options: [],
    });
    renderScreen();

    await user.click(await screen.findByRole("button", { name: "Desconciliar" }));
    await confirmPopconfirm(user, "Desconciliar");

    await waitFor(() =>
      expect(recurringCommitmentsApi.putReconciliation).toHaveBeenCalledWith(commitmentId, "2026-02-05", {
        state: "detached",
      }),
    );
    expect(recurringCommitmentsApi.deleteReconciliation).not.toHaveBeenCalled();
  });
});
