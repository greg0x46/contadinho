import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import * as scenariosApi from "../../api/scenarios";
import type { DebtLinkedTransaction, EligibleTransaction, Scenario, ScenarioDetail } from "../../api/contracts";
import { QueryTestProvider } from "../../test/QueryTestProvider";
import { DebtTimeline } from "./DebtTimeline";

vi.mock("../../api/scenarios");

const debtId = "44444444-4444-4444-8444-444444444444";
const scenarioId = "55555555-5555-4555-8555-555555555555";
const installmentId = "66666666-6666-4666-8666-666666666666";
const linkId = "77777777-7777-4777-8777-777777777777";
const candidate: EligibleTransaction = {
  id: "88888888-8888-4888-8888-888888888888",
  occurred_at: "2026-07-20T12:00:00Z",
  description: "Compra elegível",
  account_name: "Conta Corrente",
  effective_money: { value: "150", currency_code: "BRL" },
};

const scenarioSummary: Scenario = {
  id: scenarioId,
  kind: "debt_plan",
  name: "Plano de pagamento",
  debt_id: debtId,
  receivable_id: null,
  created_at: "2026-07-30T12:00:00Z",
  updated_at: "2026-07-30T12:00:00Z",
};

function baseProps(overrides: Partial<Parameters<typeof DebtTimeline>[0]> = {}) {
  return {
    debtId,
    links: [] as DebtLinkedTransaction[],
    search: "",
    onSearchChange: vi.fn(),
    candidates: [candidate],
    isSearching: false,
    isLinking: false,
    onLinkTransaction: vi.fn().mockResolvedValue({
      id: linkId,
      transaction_id: candidate.id,
      linked_amount: "150",
      linked_at: "2026-07-31T12:00:00Z",
    }),
    onUnlinkTransaction: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };
}

function renderTimeline(overrides: Partial<Parameters<typeof DebtTimeline>[0]> = {}) {
  const props = baseProps(overrides);
  render(
    <QueryTestProvider>
      <DebtTimeline {...props} />
    </QueryTestProvider>,
  );
  return props;
}

async function selectCandidate(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole("button", { name: "Adicionar pagamento" }));
  const select = await screen.findByRole("combobox", { name: "Buscar transação para vincular" });
  await user.click(select);
  const option = await screen.findByText(/Compra elegível/);
  await user.click(option);
}

describe("DebtTimeline", () => {
  it("links a transaction standalone when no debt plan exists", async () => {
    const user = userEvent.setup();
    vi.mocked(scenariosApi.listDebtScenarios).mockResolvedValue([]);
    const props = renderTimeline();

    await selectCandidate(user);
    await user.click(screen.getByRole("button", { name: "Vincular" }));

    await waitFor(() => expect(props.onLinkTransaction).toHaveBeenCalledWith(candidate.id));
    expect(scenariosApi.createRealization).not.toHaveBeenCalled();
  });

  it("links and allocates a transaction to an installment in a single action", async () => {
    const user = userEvent.setup();
    const detail: ScenarioDetail = {
      ...scenarioSummary,
      accumulated_deviation: "0.00",
      transactions: [
        {
          id: installmentId,
          scenario_id: scenarioId,
          description: "Parcela 1/3",
          amount: "150.00",
          projected_at: "2026-08-01",
          category: null,
          status: "projetada",
          realizations: [],
        },
      ],
    };
    vi.mocked(scenariosApi.listDebtScenarios).mockResolvedValue([scenarioSummary]);
    vi.mocked(scenariosApi.getScenario).mockResolvedValue(detail);
    vi.mocked(scenariosApi.createRealization).mockResolvedValue({
      ...detail.transactions[0],
      status: "paga",
    });
    const props = renderTimeline();

    await selectCandidate(user);
    const installmentSelect = await screen.findByRole("combobox", { name: "Parcela para alocar" });
    await user.click(installmentSelect);
    const parcelaOptions = await screen.findAllByText(/Parcela 1\/3/);
    await user.click(parcelaOptions[parcelaOptions.length - 1]);
    await user.click(screen.getByRole("button", { name: "Vincular" }));

    await waitFor(() => expect(props.onLinkTransaction).toHaveBeenCalledWith(candidate.id));
    await waitFor(() =>
      expect(scenariosApi.createRealization).toHaveBeenCalledWith(scenarioId, installmentId, {
        debt_link_id: linkId,
        allocated_amount: 150,
      }),
    );
  });

  it("shows an installment's allocated realizations and their Desalocar action directly, with no expand step", async () => {
    const link: DebtLinkedTransaction = {
      id: linkId,
      transaction_id: "99999999-9999-4999-8999-999999999999",
      occurred_at: "2026-07-10T12:00:00Z",
      description: "Pagamento do boleto",
      linked_amount: "150",
      current_amount: "150",
      linked_at: "2026-07-30T12:00:00Z",
    };
    const detail: ScenarioDetail = {
      ...scenarioSummary,
      accumulated_deviation: "0.00",
      transactions: [
        {
          id: installmentId,
          scenario_id: scenarioId,
          description: "Parcela 1/3",
          amount: "150.00",
          projected_at: "2026-08-01",
          category: null,
          status: "paga",
          realizations: [
            {
              id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
              debt_link_id: linkId,
              receivable_link_id: null,
              allocated_amount: "150.00",
              created_at: "2026-07-31T12:00:00Z",
            },
          ],
        },
      ],
    };
    vi.mocked(scenariosApi.listDebtScenarios).mockResolvedValue([scenarioSummary]);
    vi.mocked(scenariosApi.getScenario).mockResolvedValue(detail);
    renderTimeline({ links: [link] });

    expect(await screen.findByText(/Pagamento do boleto/)).toBeVisible();
    expect(screen.getByRole("button", { name: "Desalocar" })).toBeVisible();
  });

  it("lets a standalone-linked transaction be allocated to an installment afterwards", async () => {
    const user = userEvent.setup();
    const link: DebtLinkedTransaction = {
      id: linkId,
      transaction_id: "99999999-9999-4999-8999-999999999999",
      occurred_at: "2026-07-10T12:00:00Z",
      description: "Pagamento do boleto",
      linked_amount: "150",
      current_amount: "150",
      linked_at: "2026-07-30T12:00:00Z",
    };
    const detail: ScenarioDetail = {
      ...scenarioSummary,
      accumulated_deviation: "0.00",
      transactions: [
        {
          id: installmentId,
          scenario_id: scenarioId,
          description: "Parcela 1/3",
          amount: "150.00",
          projected_at: "2026-08-01",
          category: null,
          status: "projetada",
          realizations: [],
        },
      ],
    };
    vi.mocked(scenariosApi.listDebtScenarios).mockResolvedValue([scenarioSummary]);
    vi.mocked(scenariosApi.getScenario).mockResolvedValue(detail);
    vi.mocked(scenariosApi.createRealization).mockResolvedValue({
      ...detail.transactions[0],
      status: "paga",
    });
    renderTimeline({ links: [link] });

    expect(await screen.findByText(/Pagamento do boleto/)).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Alocar a uma parcela" }));
    const select = screen.getByRole("combobox", { name: "Parcela" });
    await user.click(select);
    const parcelaOptions = await screen.findAllByText(/Parcela 1\/3/);
    await user.click(parcelaOptions[parcelaOptions.length - 1]);
    await user.click(screen.getByRole("button", { name: "Alocar" }));

    await waitFor(() =>
      expect(scenariosApi.createRealization).toHaveBeenCalledWith(scenarioId, installmentId, {
        debt_link_id: linkId,
        allocated_amount: 150,
      }),
    );
  });

  it("deallocates a realization from an installment after confirmation", async () => {
    const user = userEvent.setup();
    const link: DebtLinkedTransaction = {
      id: linkId,
      transaction_id: "99999999-9999-4999-8999-999999999999",
      occurred_at: "2026-07-10T12:00:00Z",
      description: "Pagamento do boleto",
      linked_amount: "150",
      current_amount: "150",
      linked_at: "2026-07-30T12:00:00Z",
    };
    const realizationId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
    const detail: ScenarioDetail = {
      ...scenarioSummary,
      accumulated_deviation: "0.00",
      transactions: [
        {
          id: installmentId,
          scenario_id: scenarioId,
          description: "Parcela 1/3",
          amount: "150.00",
          projected_at: "2026-08-01",
          category: null,
          status: "paga",
          realizations: [{ id: realizationId, debt_link_id: linkId, receivable_link_id: null, allocated_amount: "150.00", created_at: "2026-07-31T12:00:00Z" }],
        },
      ],
    };
    vi.mocked(scenariosApi.listDebtScenarios).mockResolvedValue([scenarioSummary]);
    vi.mocked(scenariosApi.getScenario).mockResolvedValue(detail);
    vi.mocked(scenariosApi.deleteRealization).mockResolvedValue(undefined);
    renderTimeline({ links: [link] });

    await user.click(await screen.findByRole("button", { name: "Desalocar" }));
    const confirm = await screen.findByRole("tooltip");
    await user.click(within(confirm).getByRole("button", { name: "Desalocar" }));

    await waitFor(() =>
      expect(scenariosApi.deleteRealization).toHaveBeenCalledWith(scenarioId, installmentId, realizationId),
    );
  });
});
