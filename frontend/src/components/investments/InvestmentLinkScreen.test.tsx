import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "../../api/investmentPortfolio";
import * as legacyApi from "../../api/investments";
import type { InvestmentTransaction, TransactionItem } from "../../api/contracts";
import { QueryTestProvider } from "../../test/QueryTestProvider";
import { depositOperation, integratedAccount, manualAccount, summary, syncedPosition } from "../../test/investmentWorkspaceFixtures";
import { transactionResult } from "../../test/transactionFixtures";
import { InvestmentLinkNewOperationScreen, InvestmentLinkScreen } from "./InvestmentLinkScreen";

vi.mock("../../api/investmentPortfolio");
vi.mock("../../api/investments");

const transaction: TransactionItem = {
  ...transactionResult.items[0]!,
  effective_money: { value: "-1000", currency_code: "BRL", source: "account_currency" },
  reportable_amount: "1000",
};
const operation = { ...depositOperation, amount: "1000", occurred_on: "2026-07-17" };
const importedMovement: InvestmentTransaction = {
  id: "movement-1",
  external_id: "ext-1",
  movement_type: "BUY",
  direction: "inflow",
  quantity: null,
  value: null,
  amount: "1000",
  occurred_at: "2026-07-17T00:00:00.000000000Z",
  trade_date: "2026-07-17T00:00:00.000000000Z",
};

function show(item = transaction) {
  const onDone = vi.fn();
  const onNewOperation = vi.fn();
  render(
    <QueryTestProvider>
      <InvestmentLinkScreen transaction={item} onNewOperation={onNewOperation} onDone={onDone} />
    </QueryTestProvider>,
  );
  return { onDone, onNewOperation };
}

function openMenu(): HTMLElement {
  return document.querySelector(".ant-select-dropdown:not(.ant-select-dropdown-hidden)") as HTMLElement;
}

async function chooseOperation(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole("combobox", { name: "Movimentação de destino" }));
  await user.click(within(openMenu()).getByText(/Aporte.*Corretora/));
}

describe("InvestmentLinkScreen", () => {
  beforeEach(() => {
    vi.mocked(api.listInvestmentAccounts).mockResolvedValue([manualAccount, integratedAccount]);
    vi.mocked(api.listInvestmentPortfolios).mockResolvedValue([]);
    vi.mocked(api.listInvestmentPositions).mockResolvedValue([]);
    vi.mocked(api.listInvestmentOperations).mockResolvedValue([operation]);
    vi.mocked(api.listInvestmentReconciliations).mockResolvedValue([]);
    vi.mocked(api.getInvestmentSummary).mockResolvedValue(summary);
    vi.mocked(legacyApi.listInvestmentTransactions).mockResolvedValue([]);
  });

  it("offers the provider movement of an integrated custody as the destination", async () => {
    vi.mocked(api.listInvestmentOperations).mockResolvedValue([]);
    vi.mocked(api.listInvestmentPositions).mockResolvedValue([syncedPosition]);
    vi.mocked(legacyApi.listInvestmentTransactions).mockResolvedValue([
      importedMovement,
      { ...importedMovement, id: "movement-2", movement_type: "SELL", direction: "outflow" },
    ]);
    const user = userEvent.setup();
    const { onDone } = show();

    await user.click(await screen.findByRole("combobox", { name: "Movimentação de destino" }));
    const menu = openMenu();
    const option = await within(menu).findByText(/Aplicação.*CDB Banco Teste/);
    expect(within(menu).queryByText(/Resgate/)).not.toBeInTheDocument();
    expect(within(menu).getByText("Sugestão")).toBeInTheDocument();
    await user.click(option);
    await user.click(screen.getByRole("button", { name: "Vincular" }));

    await waitFor(() =>
      expect(api.createInvestmentReconciliation).toHaveBeenCalledWith({
        operation_id: null,
        financial_transaction_id: transaction.id,
        financial_investment_transaction_id: "movement-1",
        amount: "1000.00",
      }),
    );
    expect(legacyApi.listInvestmentTransactions).toHaveBeenCalledWith(syncedPosition.id, expect.anything());
    await waitFor(() => expect(onDone).toHaveBeenCalled());
  });

  it("hides a provider movement whose derived pivot is already fully linked", async () => {
    const pivot = {
      ...depositOperation,
      id: "pivot-1",
      account_id: integratedAccount.id,
      source: "synced" as const,
      is_editable: false,
      amount: "1000",
      notes: "BUY · CDB Banco Teste",
    };
    vi.mocked(api.listInvestmentOperations).mockResolvedValue([pivot]);
    vi.mocked(api.listInvestmentPositions).mockResolvedValue([syncedPosition]);
    vi.mocked(legacyApi.listInvestmentTransactions).mockResolvedValue([importedMovement]);
    vi.mocked(api.listInvestmentReconciliations).mockResolvedValue([
      {
        id: "link",
        operation_id: "pivot-1",
        financial_transaction_id: "other-transaction",
        financial_investment_transaction_id: "movement-1",
        amount: "1000",
        created_at: "2026-07-17T12:00:00Z",
      },
    ]);
    const user = userEvent.setup();
    show();

    await user.click(await screen.findByRole("combobox", { name: "Movimentação de destino" }));
    await waitFor(() => expect(legacyApi.listInvestmentTransactions).toHaveBeenCalled());
    const menu = openMenu();
    expect(within(menu).queryByText(/Aplicação/)).not.toBeInTheDocument();
    expect(within(menu).getByText(/Nenhuma movimentação compatível/)).toBeInTheDocument();
  });

  it("suggests nearby dates and links only the confirmed parcel", async () => {
    const user = userEvent.setup();
    show();

    await chooseOperation(user);
    await user.clear(screen.getByLabelText("Valor a vincular"));
    await user.type(screen.getByLabelText("Valor a vincular"), "600");
    await user.click(screen.getByRole("button", { name: "Vincular" }));

    await waitFor(() =>
      expect(api.createInvestmentReconciliation).toHaveBeenCalledWith({
        operation_id: operation.id,
        financial_transaction_id: transaction.id,
        financial_investment_transaction_id: null,
        amount: "600",
      }),
    );
  });

  it("uses the full bank amount even if reporting excluded the transaction", async () => {
    show({ ...transaction, reportable_amount: "0", totals_eligibility: { included: false, reason: "transfer_category" } });
    await waitFor(() => expect(screen.getByLabelText("Valor a vincular")).toHaveValue("1000,00"));
  });

  it("hands the typed amount to the Nova movimentação screen", async () => {
    const user = userEvent.setup();
    const { onNewOperation } = show();

    await waitFor(() => expect(screen.getByLabelText("Valor a vincular")).toHaveValue("1000,00"));
    await user.clear(screen.getByLabelText("Valor a vincular"));
    await user.type(screen.getByLabelText("Valor a vincular"), "130");
    await user.click(screen.getByRole("button", { name: "Nova movimentação" }));

    expect(onNewOperation).toHaveBeenCalledWith("130");
  });

  it("lists existing links with undo when the transaction is already allocated", async () => {
    vi.mocked(api.listInvestmentReconciliations).mockResolvedValue([
      {
        id: "link",
        operation_id: operation.id,
        financial_transaction_id: transaction.id,
        financial_investment_transaction_id: null,
        amount: "600",
        created_at: "2026-07-17T12:00:00Z",
      },
    ]);
    const user = userEvent.setup();
    show({ ...transaction, reportable_amount: "400", investment_transfer_amount: "600" });

    expect(await screen.findByText(/R\$ 600,00 de R\$ 1\.000,00 vinculados/)).toBeVisible();
    expect(screen.getByText("R$ 600,00 vinculado")).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Desvincular" }));
    await waitFor(() => expect(api.deleteInvestmentReconciliation).toHaveBeenCalledWith("link"));
  });

  it("refuses a currency that cannot be reconciled in reais", async () => {
    show({ ...transaction, effective_money: { value: "-1000", currency_code: "USD", source: "account_currency" } });
    expect(await screen.findByText(/não está disponível em reais/)).toBeVisible();
    expect(screen.queryByRole("button", { name: "Vincular" })).not.toBeInTheDocument();
  });
});

describe("InvestmentLinkNewOperationScreen", () => {
  beforeEach(() => {
    vi.mocked(api.listInvestmentAccounts).mockResolvedValue([manualAccount, integratedAccount]);
    vi.mocked(api.listInvestmentPortfolios).mockResolvedValue([]);
    vi.mocked(api.listInvestmentPositions).mockResolvedValue([]);
    vi.mocked(api.listInvestmentOperations).mockResolvedValue([]);
    vi.mocked(api.listInvestmentReconciliations).mockResolvedValue([]);
    vi.mocked(api.getInvestmentSummary).mockResolvedValue(summary);
  });

  it("creates and links atomically and leaves errors available for correction", async () => {
    vi.mocked(api.createInvestmentOperations).mockRejectedValue(new Error("Vínculo excede o valor disponível"));
    const user = userEvent.setup();
    render(
      <QueryTestProvider>
        <InvestmentLinkNewOperationScreen transaction={transaction} amount="1000.00" onDone={vi.fn()} />
      </QueryTestProvider>,
    );

    await waitFor(() => expect(screen.getByLabelText("Valor")).toHaveValue("1000,00"));
    await user.type(screen.getByLabelText("Observações (opcional)"), "Aporte da corretora");
    await user.click(screen.getByRole("button", { name: "Criar e vincular" }));

    await waitFor(() =>
      expect(api.createInvestmentOperations).toHaveBeenCalledWith(
        expect.objectContaining({
          reconciliation: { financial_transaction_id: transaction.id, financial_investment_transaction_id: null, amount: "1000.00" },
          operations: [expect.objectContaining({ kind: "deposit", amount: "1000.00", notes: "Aporte da corretora" })],
        }),
      ),
    );
    expect(await screen.findByText("Vínculo excede o valor disponível")).toBeVisible();
    // The error re-renders the screen with a fresh `initial`; the form must
    // still hold what was typed so the person can correct it.
    expect(screen.getByLabelText("Valor")).toHaveValue("1000,00");
    expect(screen.getByLabelText("Observações (opcional)")).toHaveValue("Aporte da corretora");
    expect(api.createInvestmentOperation).not.toHaveBeenCalled();
  });
});
