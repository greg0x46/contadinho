import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as recurringCommitmentsApi from "../../api/recurringCommitments";
import type { RecurrenceOccurrence, RecurringCommitment } from "../../api/contracts";
import { QueryTestProvider } from "../../test/QueryTestProvider";
import { RecurrenceOccurrenceList } from "./RecurrenceOccurrenceList";

vi.mock("../../api/recurringCommitments");

const commitmentId = "22222222-2222-4222-8222-222222222222";
const transactionId = "11111111-1111-4111-8111-111111111111";

const commitment: RecurringCommitment = {
  id: commitmentId,
  name: "Aluguel",
  kind: "expense",
  amount: "1500.00",
  category_id: "44444444-4444-4444-8444-444444444444",
  account_id: null,
  cadence: "monthly",
  day_of_month: 5,
  month_of_year: null,
  start_date: "2026-01-01",
  end_date: null,
  is_active: true,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const reconciledByRule: RecurrenceOccurrence = {
  date: "2026-02-05",
  expected_amount: "1500.00",
  status: "reconciled",
  origin: "rule",
  transaction: {
    id: transactionId,
    occurred_at: "2026-02-06T12:00:00Z",
    description: "ALUGUEL IMOBILIARIA",
    account_name: "Conta Corrente",
    effective_money: { value: "1490.00", currency_code: "BRL" },
  },
};

const unreconciled: RecurrenceOccurrence = {
  date: "2026-01-05",
  expected_amount: "1500.00",
  status: "unreconciled",
  origin: null,
  transaction: null,
};

const detached: RecurrenceOccurrence = {
  date: "2026-03-05",
  expected_amount: "1500.00",
  status: "detached",
  origin: null,
  transaction: null,
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

function renderList() {
  return render(
    <QueryTestProvider>
      <RecurrenceOccurrenceList commitment={commitment} />
    </QueryTestProvider>,
  );
}

// The row a given occurrence's actions live in — the table has one row per
// occurrence and several share button labels, so assertions have to scope.
async function rowFor(dateLabel: string) {
  const cell = await screen.findByText(dateLabel);
  const row = cell.closest("tr");
  if (row === null) throw new Error(`no row for ${dateLabel}`);
  return within(row);
}

describe("RecurrenceOccurrenceList", () => {
  beforeEach(() => {
    vi.mocked(recurringCommitmentsApi.putReconciliation).mockResolvedValue(unreconciled);
    vi.mocked(recurringCommitmentsApi.deleteReconciliation).mockResolvedValue(undefined);
    vi.mocked(recurringCommitmentsApi.listReconciliationCandidates).mockResolvedValue([]);
  });

  it("shows which transaction settled each occurrence and how it was decided", async () => {
    vi.mocked(recurringCommitmentsApi.listRecurrenceOccurrences).mockResolvedValue([
      reconciledByRule,
      unreconciled,
    ]);
    renderList();

    expect(await screen.findByText("Conciliada (automática)")).toBeVisible();
    expect(screen.getByText(/ALUGUEL IMOBILIARIA/)).toBeVisible();
    expect(screen.getByText("Não conciliada")).toBeVisible();
  });

  it("desconciliar records a detached decision so the rule cannot re-claim it", async () => {
    const user = userEvent.setup();
    vi.mocked(recurringCommitmentsApi.listRecurrenceOccurrences).mockResolvedValue([reconciledByRule]);
    renderList();

    const row = await rowFor("05/02/2026");
    await user.click(row.getByRole("button", { name: "Desconciliar" }));
    await confirmPopconfirm(user, "Desconciliar");

    await waitFor(() =>
      expect(recurringCommitmentsApi.putReconciliation).toHaveBeenCalledWith(
        commitmentId,
        "2026-02-05",
        { state: "detached" },
      ),
    );
  });

  it("offers voltar ao automático only on a detached occurrence", async () => {
    const user = userEvent.setup();
    vi.mocked(recurringCommitmentsApi.listRecurrenceOccurrences).mockResolvedValue([
      detached,
      unreconciled,
    ]);
    renderList();

    const untouched = await rowFor("05/01/2026");
    expect(untouched.queryByRole("button", { name: "Voltar ao automático" })).toBeNull();

    const detachedRow = await rowFor("05/03/2026");
    await user.click(detachedRow.getByRole("button", { name: "Voltar ao automático" }));

    await waitFor(() =>
      expect(recurringCommitmentsApi.deleteReconciliation).toHaveBeenCalledWith(
        commitmentId,
        "2026-03-05",
      ),
    );
  });

  it("reconciles an occurrence with a hand-picked transaction", async () => {
    const user = userEvent.setup();
    vi.mocked(recurringCommitmentsApi.listRecurrenceOccurrences).mockResolvedValue([unreconciled]);
    vi.mocked(recurringCommitmentsApi.listReconciliationCandidates).mockResolvedValue([
      {
        id: transactionId,
        occurred_at: "2026-01-06T12:00:00Z",
        description: "ALUGUEL IMOBILIARIA",
        account_name: "Conta Corrente",
        effective_money: { value: "1490.00", currency_code: "BRL" },
      },
    ]);
    renderList();

    const row = await rowFor("05/01/2026");
    await user.click(row.getByRole("button", { name: "Conciliar com…" }));

    await user.click(await screen.findByRole("combobox", { name: "Buscar transação para conciliar" }));
    await user.click(await screen.findByText(/ALUGUEL IMOBILIARIA/));
    await user.click(screen.getByRole("button", { name: "Conciliar" }));

    await waitFor(() =>
      expect(recurringCommitmentsApi.putReconciliation).toHaveBeenCalledWith(
        commitmentId,
        "2026-01-05",
        { state: "linked", transaction_id: transactionId },
      ),
    );
  });
});
