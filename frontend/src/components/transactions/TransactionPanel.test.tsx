import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as investmentPortfolioApi from "../../api/investmentPortfolio";
import * as recurringCommitmentsApi from "../../api/recurringCommitments";
import * as transactionsApi from "../../api/transactions";
import type { Category } from "../../api/contracts";
import { QueryTestProvider } from "../../test/QueryTestProvider";
import { transactionResult } from "../../test/transactionFixtures";
import { TransactionPanel } from "./TransactionPanel";

vi.mock("../../api/transactions");
vi.mock("../../api/recurringCommitments");
vi.mock("../../api/investmentPortfolio");
vi.mock("../../api/investments");

function renderPanel(ui: ReactElement) {
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

const item = transactionResult.items[0]!;

async function openMenu(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Mais ações" }));
  return await screen.findByRole("menu");
}

describe("TransactionPanel", () => {
  beforeEach(() => {
    vi.mocked(transactionsApi.getTransactionReconciliation).mockResolvedValue({ current: null, options: [] });
    vi.mocked(investmentPortfolioApi.listInvestmentAccounts).mockResolvedValue([]);
    vi.mocked(investmentPortfolioApi.listInvestmentPortfolios).mockResolvedValue([]);
    vi.mocked(investmentPortfolioApi.listInvestmentPositions).mockResolvedValue([]);
    vi.mocked(investmentPortfolioApi.listInvestmentOperations).mockResolvedValue([]);
    vi.mocked(investmentPortfolioApi.listInvestmentReconciliations).mockResolvedValue([]);
  });

  it("opens on the overview with identity, category and the totals switch", () => {
    renderPanel(<TransactionPanel item={item} categories={activeCategories} onClose={vi.fn()} />);

    expect(screen.getByRole("heading", { name: "Transação" })).toBeVisible();
    expect(screen.getByText("Mercado")).toBeVisible();
    expect(screen.getByText("Conta corrente · Banco Teste")).toBeVisible();
    expect(screen.getByRole("button", { name: "Categoria" })).toHaveTextContent("Alimentação");
    expect(screen.getByRole("switch", { name: "Considerar nos totais" })).toBeChecked();
    // Advanced sections are rows, not open content.
    expect(screen.queryByText("Identificador externo")).not.toBeInTheDocument();
  });

  it("assigns a category manually", async () => {
    const user = userEvent.setup();
    const onCategory = vi.fn();
    renderPanel(
      <TransactionPanel item={item} categories={activeCategories} onClose={vi.fn()} onCategory={onCategory} />,
    );

    await user.click(screen.getByRole("button", { name: "Categoria" }));
    await user.click(await screen.findByRole("option", { name: "Transporte" }));

    expect(onCategory).toHaveBeenCalledWith(item.id, "44444444-4444-4444-8444-444444444444");
  });

  it("offers the bank's suggestion with a one-tap Usar when it matches a category", async () => {
    const user = userEvent.setup();
    const onCategory = vi.fn();
    renderPanel(
      <TransactionPanel
        item={{ ...item, source_category: "transporte", internal_category: null }}
        categories={activeCategories}
        onClose={vi.fn()}
        onCategory={onCategory}
      />,
    );

    expect(screen.getByText(/Sugestão:/)).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Usar" }));
    expect(onCategory).toHaveBeenCalledWith(item.id, "44444444-4444-4444-8444-444444444444");
  });

  it("shows no Usar when the suggestion is already the category or matches nothing", () => {
    const { rerender } = renderPanel(
      <TransactionPanel item={item} categories={activeCategories} onClose={vi.fn()} onCategory={vi.fn()} />,
    );
    expect(screen.queryByRole("button", { name: "Usar" })).not.toBeInTheDocument();

    rerender(
      <MemoryRouter>
        <QueryTestProvider>
          <TransactionPanel
            item={{ ...item, source_category: "Pets" }}
            categories={activeCategories}
            onClose={vi.fn()}
            onCategory={vi.fn()}
          />
        </QueryTestProvider>
      </MemoryRouter>,
    );
    expect(screen.queryByText(/Sugestão:/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Usar" })).not.toBeInTheDocument();
  });

  it("warns when the vigente category is inactive without blocking display", () => {
    renderPanel(
      <TransactionPanel
        item={{ ...item, internal_category: { ...item.internal_category!, is_active: false } }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText(/continua vigente para esta transação/)).toBeVisible();
  });

  it("turns the totals switch off through onInclusion and explains the consequence when off", async () => {
    const user = userEvent.setup();
    const onInclusion = vi.fn();
    const { rerender } = renderPanel(
      <TransactionPanel item={item} categories={activeCategories} onClose={vi.fn()} onInclusion={onInclusion} />,
    );

    await user.click(screen.getByRole("switch", { name: "Considerar nos totais" }));
    expect(onInclusion).toHaveBeenCalledWith(item.id, "ignored");
    expect(screen.queryByText(/não será considerada nos relatórios/)).not.toBeInTheDocument();

    rerender(
      <MemoryRouter>
        <QueryTestProvider>
          <TransactionPanel
            item={{
              ...item,
              inclusion: { state: "ignored", changed_at: null, origin: "manual", rule_name: null },
              totals_eligibility: { included: false, reason: "ignored" },
            }}
            categories={activeCategories}
            onClose={vi.fn()}
            onInclusion={onInclusion}
          />
        </QueryTestProvider>
      </MemoryRouter>,
    );
    expect(screen.getByRole("switch", { name: "Considerar nos totais" })).not.toBeChecked();
    expect(screen.getByText(/não será considerada nos relatórios/)).toBeVisible();
    expect(screen.getByRole("button", { name: /Conciliar com recorrência/ })).toBeDisabled();
  });

  it("navigates to Informações técnicas and back within the same panel", async () => {
    const user = userEvent.setup();
    renderPanel(
      <TransactionPanel
        item={{ ...item, totals_eligibility: { included: false, reason: "unclassified" } }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );

    await user.click(screen.getByText("Informações técnicas"));
    expect(screen.getByRole("heading", { name: "Informações técnicas" })).toBeVisible();
    expect(screen.getByText("Fora dos totais: tipo não classificado")).toBeVisible();
    expect(screen.getAllByRole("dialog")).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "Voltar" }));
    expect(screen.getByRole("heading", { name: "Transação" })).toBeVisible();
    expect(screen.getByText("Mercado")).toBeVisible();
  });

  it("keeps card and installment details one level down", async () => {
    const user = userEvent.setup();
    renderPanel(
      <TransactionPanel
        item={{ ...item, card: { number: "**** 1234", installment_number: 3, total_installments: 12 } }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText("**** 1234 · Parcela 3/12")).toBeVisible();

    await user.click(screen.getByText("Detalhes da transação"));
    expect(screen.getByText("Cartão")).toBeVisible();
    expect(screen.getByText("Parcela 3/12")).toBeVisible();
  });

  it("does not offer Vincular a investimento for a card purchase", () => {
    renderPanel(
      <TransactionPanel
        item={{ ...item, card: { number: "**** 4321", installment_number: null, total_installments: null } }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.queryByRole("button", { name: /Vincular a investimento/ })).not.toBeInTheDocument();
  });

  it("describes the remainder of a linked aporte as spending on the overview", () => {
    renderPanel(
      <TransactionPanel
        item={{ ...item, investment_transfer_amount: "100.0000", reportable_amount: "23.4500" }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText(/aporte vinculado: R\$\s100,00 · restante nos gastos: R\$\s23,45/)).toBeVisible();
    expect(screen.getByText(/R\$\s100,00 vinculados/)).toBeVisible();
  });

  it("describes the remainder of a linked resgate as income", () => {
    renderPanel(
      <TransactionPanel
        item={{
          ...item,
          classification: "inflow",
          movement_type: "CREDIT",
          investment_transfer_amount: "100.0000",
          reportable_amount: "23.4500",
        }}
        categories={activeCategories}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText(/resgate vinculado: R\$\s100,00 · restante nas receitas: R\$\s23,45/)).toBeVisible();
  });

  it("says an ignored linked line is out of the totals instead of showing a zero remainder", () => {
    renderPanel(
      <TransactionPanel
        item={{
          ...item,
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

  it("opens Vincular a investimento as a screen of the same panel, not a second drawer", async () => {
    const user = userEvent.setup();
    renderPanel(<TransactionPanel item={item} categories={activeCategories} onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /Vincular a investimento/ }));
    expect(screen.getByRole("heading", { name: "Vincular a investimento" })).toBeVisible();
    expect(await screen.findByRole("button", { name: "Vincular" })).toBeVisible();
    expect(screen.getAllByRole("dialog")).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "Nova movimentação" }));
    expect(screen.getByRole("heading", { name: "Nova movimentação" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Criar e vincular" })).toBeVisible();
    expect(screen.getAllByRole("dialog")).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "Voltar" }));
    expect(screen.getByRole("heading", { name: "Vincular a investimento" })).toBeVisible();
  });

  it("creates a recurrence from the line inside the panel and returns to the overview", async () => {
    const user = userEvent.setup();
    vi.mocked(recurringCommitmentsApi.createRecurringCommitment).mockResolvedValue({
      id: "c1",
      name: "Mercado",
      kind: "expense",
      amount: "123.45",
      category_id: activeCategories[0]!.id,
      account_id: null,
      cadence: "monthly",
      day_of_month: 15,
      month_of_year: null,
      start_date: "2026-07-15",
      end_date: null,
      is_active: true,
      created_at: "2026-07-15T00:00:00Z",
      updated_at: "2026-07-15T00:00:00Z",
    });
    renderPanel(<TransactionPanel item={item} categories={activeCategories} onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Criar recorrência" }));
    expect(screen.getByRole("heading", { name: "Criar recorrência" })).toBeVisible();
    expect(screen.getByLabelText("Nome")).toHaveValue("Mercado");
    expect(screen.getByLabelText("Valor")).toHaveValue("123,45");

    await user.click(screen.getByRole("button", { name: "Criar recorrência" }));
    await waitFor(() =>
      expect(recurringCommitmentsApi.createRecurringCommitment).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Mercado", amount: "123.45", day_of_month: 15 }),
      ),
    );
    expect(await screen.findByRole("heading", { name: "Transação" })).toBeVisible();
  });

  it("keeps synced transactions free of manual-only actions", () => {
    renderPanel(<TransactionPanel item={{ ...item, origin: "synced" }} categories={activeCategories} onClose={vi.fn()} />);
    expect(screen.queryByText("Manual")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Mais ações" })).not.toBeInTheDocument();
  });

  it("edits a manual transaction in place and deletes it behind a dialog", async () => {
    const user = userEvent.setup();
    const onSaveManual = vi.fn().mockResolvedValue(undefined);
    const onDeleteManual = vi.fn();
    renderPanel(
      <TransactionPanel
        item={{ ...item, origin: "manual" }}
        categories={activeCategories}
        accounts={[{ id: item.account.id, name: "Conta corrente", institution: "Banco Teste", currency_code: "BRL" } as never]}
        onClose={vi.fn()}
        onSaveManual={onSaveManual}
        onDeleteManual={onDeleteManual}
      />,
    );
    expect(screen.getByText("Manual")).toBeVisible();

    let menu = await openMenu(user);
    await user.click(within(menu).getByText("Editar lançamento"));
    expect(screen.getByRole("heading", { name: "Editar lançamento" })).toBeVisible();
    expect(screen.getByLabelText("Descrição")).toHaveValue("Mercado");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(onSaveManual).toHaveBeenCalledWith(item.id, expect.objectContaining({ description: "Mercado" })));
    expect(await screen.findByRole("heading", { name: "Transação" })).toBeVisible();

    menu = await openMenu(user);
    await user.click(within(menu).getByText("Excluir lançamento"));
    await screen.findByText("Esta ação não pode ser desfeita.");
    const dialog = document.querySelector<HTMLElement>(".ant-modal-content")!;
    await user.click(within(dialog).getByRole("button", { name: "Excluir" }));
    expect(onDeleteManual).toHaveBeenCalledWith(item.id);
  });

  it("shows the delete error when a manual delete is rejected", () => {
    renderPanel(
      <TransactionPanel
        item={{ ...item, origin: "manual" }}
        categories={activeCategories}
        onClose={vi.fn()}
        onDeleteManual={vi.fn()}
        deleteManualError="Não foi possível excluir o lançamento."
      />,
    );
    expect(screen.getByText("Não foi possível excluir o lançamento.")).toBeVisible();
  });
});
