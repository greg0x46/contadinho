import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as accountsApi from "../api/accounts";
import * as investmentsApi from "../api/investments";
import * as workspaceApi from "../api/investmentPortfolio";
import { QueryTestProvider } from "../test/QueryTestProvider";
import {
  buyOperation,
  closedPosition,
  depositOperation,
  goal,
  goalId,
  integratedAccount,
  manualAccount,
  manualPosition,
  summary,
  syncedPosition,
  syncedPositionId,
  withdrawalOperation,
} from "../test/investmentWorkspaceFixtures";
import { InvestmentsPage } from "./InvestmentsPage";

vi.mock("../api/investmentPortfolio");
vi.mock("../api/investments");
vi.mock("../api/accounts");

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <InvestmentsPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

type User = ReturnType<typeof userEvent.setup>;

async function openGoalView(user: User) {
  await user.click(await screen.findByText("Por objetivo"));
}

/**
 * antd renders Select options in a portal, and a goal name also shows up as
 * the selected value of other rows, so the click is scoped to the open menu.
 */
async function pickOption(user: User, label: string) {
  const dropdown = document.querySelector(".ant-select-dropdown:not(.ant-select-dropdown-hidden)");
  if (dropdown === null) throw new Error("Nenhum menu de seleção está aberto.");
  await user.click(within(dropdown as HTMLElement).getByTitle(label));
}

describe("InvestmentsPage", () => {
  beforeEach(() => {
    vi.mocked(workspaceApi.listInvestmentAccounts).mockResolvedValue([manualAccount, integratedAccount]);
    vi.mocked(workspaceApi.listInvestmentPortfolios).mockResolvedValue([goal]);
    vi.mocked(workspaceApi.listInvestmentPositions).mockResolvedValue([manualPosition, syncedPosition]);
    vi.mocked(workspaceApi.listInvestmentOperations).mockResolvedValue([
      depositOperation,
      buyOperation,
      withdrawalOperation,
    ]);
    vi.mocked(workspaceApi.listInvestmentReconciliations).mockResolvedValue([]);
    vi.mocked(workspaceApi.getInvestmentSummary).mockResolvedValue(summary);
    vi.mocked(investmentsApi.listInvestments).mockResolvedValue([]);
    vi.mocked(accountsApi.listAccounts).mockResolvedValue([]);
  });

  it("shows each custody account with its value, movements, cash and last update", async () => {
    renderPage();

    expect(await screen.findByText("Corretora XP")).toBeVisible();
    expect(screen.getAllByText("R$ 1.700,00").length).toBeGreaterThan(0);
    // Aportes and resgates come from the account's own cash operations.
    expect(screen.getAllByText("R$ 2.000,00").length).toBeGreaterThan(0);
    expect(screen.getAllByText("R$ 300,00").length).toBeGreaterThan(0);
    expect(screen.getAllByText("R$ 500,00").length).toBeGreaterThan(0);
    expect(screen.getByText("12/09/2026")).toBeVisible();
    expect(screen.getByText("Tesouro Selic 2029")).toBeVisible();
    // An imported account is read-only apart from the goal of its positions.
    expect(screen.getByText("Conta integrada")).toBeVisible();
    expect(screen.getByText("Somente o objetivo")).toBeVisible();
  });

  it("hides closed positions by default and reveals them through the switch", async () => {
    vi.mocked(workspaceApi.listInvestmentPositions).mockResolvedValue([
      manualPosition,
      syncedPosition,
      closedPosition,
    ]);
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("Tesouro Selic 2029");
    expect(screen.queryByText("LCI Banco Antigo")).not.toBeInTheDocument();

    await user.click(screen.getByRole("switch", { name: "Mostrar posições fechadas" }));

    expect(await screen.findByText("LCI Banco Antigo")).toBeVisible();
    expect(screen.getByText("Encerrada")).toBeVisible();
  });

  it("names a closed position in the operations table while the switch hides it", async () => {
    vi.mocked(workspaceApi.listInvestmentPositions).mockResolvedValue([manualPosition, syncedPosition, closedPosition]);
    vi.mocked(workspaceApi.listInvestmentOperations).mockResolvedValue([
      depositOperation,
      { ...buyOperation, id: "op-closed-buy", position_id: closedPosition.id, amount: "500.00", unit_price: "500.00" },
    ]);
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("Tesouro Selic 2029");
    await user.click(screen.getByText("Movimentações (2)"));

    const table = await screen.findByLabelText("Movimentações");
    expect(within(table).getByText("LCI Banco Antigo")).toBeVisible();
    expect(within(table).queryByText("Posição removida")).not.toBeInTheDocument();
    // The positions table still hides it.
    expect(screen.getAllByText("LCI Banco Antigo")).toHaveLength(1);
  });

  it("counts a fully sold position in the goal's compras e vendas regardless of the switch", async () => {
    const soldOut = { ...closedPosition, portfolio_id: goalId };
    vi.mocked(workspaceApi.listInvestmentPositions).mockResolvedValue([manualPosition, syncedPosition, soldOut]);
    vi.mocked(workspaceApi.listInvestmentOperations).mockResolvedValue([
      { ...buyOperation, id: "op-sold-buy", position_id: soldOut.id, amount: "1000.00", occurred_on: "2026-08-01" },
      { ...buyOperation, id: "op-sold-sell", kind: "sell", position_id: soldOut.id, amount: "1000.00", occurred_on: "2026-08-15" },
    ]);
    const user = userEvent.setup();
    renderPage();
    await openGoalView(user);

    // The goal name also shows as the selected value of position rows, so
    // the card is found through its title (a plain span, not a select item).
    const goalCard = (await screen.findByText("Reserva de emergência", { selector: "span:not([class])" })).closest(".ant-card")!;
    expect(within(goalCard as HTMLElement).queryByText("LCI Banco Antigo")).not.toBeInTheDocument();
    const figure = (label: string) => within(goalCard as HTMLElement).getByText(label).closest(".ant-flex") as HTMLElement;
    expect(within(figure("Compras e saldo inicial")).getByText("R$ 1.000,00")).toBeVisible();
    expect(within(figure("Vendas")).getByText("R$ 1.000,00")).toBeVisible();
  });

  it("does not invent a rentabilidade without cotação or acquisition cost", async () => {
    renderPage();

    await screen.findByText("CDB Banco Teste");
    expect(screen.getAllByText("Rentabilidade indisponível").length).toBe(1);
    // The manual position does have both, so its gain is a real figure.
    expect(screen.getByText("R$ 200,00")).toBeVisible();
  });

  it("groups value by goal and says goals never add to the total", async () => {
    const user = userEvent.setup();
    renderPage();
    await openGoalView(user);

    expect((await screen.findAllByText("Reserva de emergência")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("R$ 1.200,00").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Sem objetivo").length).toBeGreaterThan(0);
    expect(screen.getByText("Objetivos apenas agrupam valor")).toBeVisible();
    expect(screen.getByText(/não soma nada ao total investido/)).toBeVisible();
  });

  it("creates a manual custody account through the form", async () => {
    const user = userEvent.setup();
    vi.mocked(workspaceApi.createInvestmentAccount).mockResolvedValue(manualAccount);
    renderPage();

    await user.click(await screen.findByRole("button", { name: /Nova conta de custódia/ }));
    await user.type(screen.getByLabelText("Nome"), "Corretora nova");
    await user.click(screen.getByRole("button", { name: "Criar conta" }));

    await waitFor(() =>
      expect(workspaceApi.createInvestmentAccount).toHaveBeenCalledWith({
        name: "Corretora nova",
        currency_code: "BRL",
        financial_account_id: null,
      }),
    );
  });

  it("registers an aporte through the operation form", async () => {
    const user = userEvent.setup();
    vi.mocked(workspaceApi.createInvestmentOperation).mockResolvedValue(depositOperation);
    renderPage();

    await user.click(await screen.findByRole("button", { name: "Registrar movimentação" }));
    await user.type(screen.getByLabelText("Valor"), "250");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() => expect(workspaceApi.createInvestmentOperation).toHaveBeenCalled());
    const write = vi.mocked(workspaceApi.createInvestmentOperation).mock.calls[0]![0];
    expect(write.account_id).toBe(manualAccount.id);
    expect(write.kind).toBe("deposit");
    expect(write.amount).toBe("250");
  });

  it("assigns a goal to a synced position without moving any money", async () => {
    const user = userEvent.setup();
    vi.mocked(workspaceApi.updateInvestmentPosition).mockResolvedValue({
      ...syncedPosition,
      portfolio_id: goalId,
    });
    renderPage();

    await screen.findByText("CDB Banco Teste");
    await user.click(screen.getByRole("combobox", { name: "Objetivo de CDB Banco Teste" }));
    await pickOption(user, "Reserva de emergência");

    await waitFor(() =>
      expect(workspaceApi.updateInvestmentPosition).toHaveBeenCalledWith(syncedPositionId, {
        name: "CDB Banco Teste",
        ticker: null,
        asset_type: "CDB",
        portfolio_id: goalId,
        notes: null,
      }),
    );
    // Nothing about the money is part of the write, and the figures on screen
    // are the same ones the workspace already reported.
    expect(screen.getAllByText("R$ 300,00").length).toBeGreaterThan(0);
    expect(screen.getAllByText("R$ 2.000,00").length).toBeGreaterThan(0);
  });

  it("keeps the provider-imported listing reachable", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByText("Sincronizados"));

    expect(await screen.findByText("Importados da sua instituição")).toBeVisible();
    const table = screen.getByLabelText("Investimentos");
    expect(within(table).getByText(/Nenhum investimento sincronizado ainda/)).toBeVisible();
  });
  it("creates an empty position so a new purchase does not invent an opening asset", async () => {
    const user = userEvent.setup();
    vi.mocked(workspaceApi.createInvestmentPosition).mockResolvedValue(manualPosition);
    renderPage();
    await screen.findByText("Corretora XP");
    await user.click(screen.getAllByRole("button", { name: "Nova posição" })[0]!);
    await user.type(screen.getByLabelText("Nome"), "Novo CDB");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(workspaceApi.createInvestmentPosition).toHaveBeenCalled());
    expect(vi.mocked(workspaceApi.createInvestmentPosition).mock.calls[0]![0].name).toBe("Novo CDB");
  });

  it("records an integrated deposit without requiring a manual account", async () => {
    vi.mocked(workspaceApi.listInvestmentAccounts).mockResolvedValue([integratedAccount]);
    vi.mocked(workspaceApi.createInvestmentOperation).mockResolvedValue({ ...depositOperation, account_id: integratedAccount.id });
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Conta integrada");
    await user.click(screen.getAllByRole("button", { name: "Registrar movimentação" })[0]!);
    await user.type(screen.getByLabelText("Valor"), "1000");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(workspaceApi.createInvestmentOperation).toHaveBeenCalledWith(expect.objectContaining({account_id: integratedAccount.id, kind:"deposit",amount:"1000"})));
  });

  it("registers reinvested income and its purchase in one request", async () => {
    vi.mocked(workspaceApi.createInvestmentOperations).mockResolvedValue([depositOperation, buyOperation]);
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Corretora XP");
    await user.click(screen.getAllByRole("button", { name: "Registrar movimentação" })[0]!);
    await user.click(screen.getByLabelText("Tipo"));
    await pickOption(user,"Compra");
    await user.click(screen.getByLabelText(/Posição/));
    await pickOption(user,manualPosition.name);
    await user.type(screen.getByLabelText("Quantidade"),"2");
    await user.type(screen.getByLabelText("Preço unitário"),"10");
    await user.click(screen.getByRole("checkbox",{name:"Registrar entrada de dinheiro junto com a compra"}));
    await user.click(screen.getByRole("combobox",{name:"Origem do dinheiro"}));
    await pickOption(user,"Rendimento reinvestido");
    await user.click(screen.getByRole("button",{name:"Salvar"}));
    await waitFor(()=>expect(workspaceApi.createInvestmentOperations).toHaveBeenCalledWith({operations:[
      expect.objectContaining({kind:"income",amount:"20.00"}),
      expect.objectContaining({kind:"buy",quantity:"2",unit_price:"10"}),
    ]}));
  });

});
