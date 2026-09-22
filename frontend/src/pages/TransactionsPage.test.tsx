import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import { TransactionsPage } from "./TransactionsPage";
import { QueryTestProvider } from "../test/QueryTestProvider";
import {
  accountId,
  categoryId,
  ignoredTransactionResult,
  transactionId,
  transactionJsonResponse,
  transactionResult,
} from "../test/transactionFixtures";

describe("TransactionsPage", () => {
  const renderPage = (entry = "/transacoes") =>
    render(
      <MemoryRouter initialEntries={[entry]}>
        <QueryTestProvider>
          <TransactionsPage />
        </QueryTestProvider>
      </MemoryRouter>,
    );

  it("loads credit card links with the filter selected and allows removing it", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(transactionJsonResponse());
    renderPage("/transacoes?credit_card=true");
    expect(screen.queryByRole("checkbox", { name: "Cartão de crédito" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Filtros 1" }));
    const checkbox = await screen.findByRole("checkbox", { name: "Cartão de crédito" });
    expect(checkbox).toBeChecked();
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body)).filters.credit_card).toBe(true);
    await user.click(checkbox);
    await user.click(screen.getByRole("button", { name: "Aplicar" }));
    await waitFor(() => {
      const queries = fetchMock.mock.calls.filter(([url]) => String(url).includes("/transactions/query"));
      expect(JSON.parse(String(queries.at(-1)?.[1]?.body)).filters.credit_card).toBeNull();
    });
    expect(checkbox).not.toBeChecked();
  });

  it("renders compact BRL totals, translated facts and query state", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(transactionJsonResponse());
    renderPage();
    expect(await screen.findByRole("heading", { name: "Transações" })).toBeVisible();
    expect(await screen.findByText("Mercado", {}, { timeout: 10_000 })).toBeVisible();
    // The provider status is no longer a row-level fact: e1c8e5a moved it
    // into the detail drawer's technical information, where
    // TransactionDetailDrawer.test.tsx covers it. "Saídas" below is the
    // translated fact this list still owns.
    expect(screen.getAllByText("Saídas").length).toBeGreaterThan(0);
    expect(screen.getAllByText(/R\$/).length).toBeGreaterThan(0);
    expect(screen.queryByText("USD")).not.toBeInTheDocument();
    const request = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(request.group_by).toBe("week");
    expect(request.filters.date_from).toMatch(/^\d{4}-\d{2}-01$/);
    expect(request.filters.date_to).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  it("distinguishes loading from an empty or failed result", () => {
    vi.spyOn(globalThis, "fetch").mockReturnValue(new Promise(() => undefined));
    renderPage();
    expect(screen.getByRole("status")).toHaveTextContent("Carregando transações");
    expect(screen.queryByText(/não há transações/i)).not.toBeInTheDocument();
  });

  it.each([
    [0, "Ainda não há transações armazenadas."],
    [3, "Não há transações no mês atual."],
  ])("renders contextual empty states", async (storedTotal, message) => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      transactionJsonResponse({
        ...transactionResult,
        stored_total: storedTotal,
        page: { ...transactionResult.page, total_items: 0, total_pages: 0 },
        items: [],
        totals: [],
        groups: [],
      }),
    );
    renderPage();
    expect(await screen.findByText(message)).toBeVisible();
    const summary = screen.getByRole("region", { name: "Resumo financeiro" });
    expect(within(summary).getByText("Resultado do período")).toBeVisible();
    expect(within(summary).getAllByText(/R\$\s*0,00/)).toHaveLength(3);
  });

  it("renders initial unavailability with manual retry", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      transactionJsonResponse(
        { type: "/problems/query", title: "Indisponível", status: 503 },
        { status: 503, headers: { "content-type": "application/problem+json" } },
      ),
    );
    renderPage();
    expect(await screen.findByText("Não foi possível carregar as transações")).toBeVisible();
    expect(screen.getByRole("button", { name: "Tentar novamente" })).toBeVisible();
  });

  it("applies a quick filter and renders the filtered empty state", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockImplementation(async () =>
      transactionJsonResponse({
        ...transactionResult,
        stored_total: 3,
        page: { ...transactionResult.page, total_items: 0, total_pages: 0 },
        items: [],
        totals: [],
        groups: [],
      }),
    );
    renderPage();
    await screen.findByText("Não há transações no mês atual.");
    await user.type(screen.getByLabelText("Descrição"), "inexistente");
    expect(
      await screen.findByText(
        "Nenhum resultado encontrado para os filtros selecionados.",
        {},
        { timeout: 10_000 },
      ),
    ).toBeVisible();
  });

  it("clears filters while keeping the selected period and resets pagination", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(transactionJsonResponse());
    renderPage("/transacoes?date_from=2024-01-01&date_to=2024-12-31&description=Mercado&page=2");
    await screen.findByText("1 transação");
    await user.click(screen.getByRole("button", { name: "Limpar filtros" }));
    await waitFor(() => {
      const requests = fetchMock.mock.calls.flatMap(([, init]) => init?.body ? [JSON.parse(String(init.body))] : []);
      expect(requests.at(-1)).toMatchObject({ page: 1, filters: {
        date_from: "2024-01-01", date_to: "2024-12-31", description: null,
      } });
    });
  });

  it("restores filters and grouping from the URL, including older single-value links", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(transactionJsonResponse());
    renderPage(
      `/transacoes?description=Mercado&classification=inflow&account_id=${accountId}&category_ids=${categoryId},${categoryId}&group=day`,
    );
    await screen.findByText("Mercado");
    const request = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(request.group_by).toBe("day");
    expect(request.filters).toMatchObject({
      description: "Mercado",
      classification: "inflow",
      account_ids: [accountId],
      category_ids: [categoryId],
    });
    // Three filter groups are active, whatever the number of values.
    expect(screen.getByRole("button", { name: "Filtros 3" })).toBeVisible();
  });

  it("refreshes the complete backend snapshot after ignore without changing structural counts", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
      if (init?.method === "PUT") {
        return transactionJsonResponse({
          transaction_id: transactionId,
          state: "ignored",
          changed_at: "2026-07-30T13:00:00Z",
        });
      }
      return transactionJsonResponse(
        fetchMock.mock.calls.some(([, options]) => options?.method === "PUT")
          ? ignoredTransactionResult
          : transactionResult,
      );
    });
    renderPage();
    await screen.findByText("Mercado");
    expect(screen.getByText("1 transação")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Ações de Mercado" }));
    await user.click(await screen.findByRole("menuitem", { name: "Ignorar" }));
    expect(
      await screen.findByText("Transação ignorada. Totais atualizados."),
    ).toBeInTheDocument();
    expect(screen.getByText("Ignorada")).toBeVisible();
    expect(screen.getByText("1 transação")).toBeVisible();
    expect(screen.getByText(/sem transações ignoradas/)).toBeVisible();
  });

  it("keeps a persistent error and retries the same explicit target", async () => {
    const user = userEvent.setup();
    let putAttempts = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
      if (init?.method === "PUT") {
        putAttempts += 1;
        if (putAttempts === 1) {
          return transactionJsonResponse(
            { type: "/problems/write", title: "Indisponível", status: 503 },
            { status: 503, headers: { "content-type": "application/problem+json" } },
          );
        }
        return transactionJsonResponse({
          transaction_id: transactionId,
          state: "ignored",
          changed_at: "2026-07-30T13:00:00Z",
        });
      }
      return transactionJsonResponse(putAttempts > 1 ? ignoredTransactionResult : transactionResult);
    });
    renderPage();
    await screen.findByText("Mercado");
    await user.click(screen.getByRole("button", { name: "Ações de Mercado" }));
    await user.click(await screen.findByRole("menuitem", { name: "Ignorar" }));
    expect(await screen.findByText("Não foi possível salvar a decisão")).toBeVisible();
    expect(screen.getByText("Mercado")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(await screen.findByText("Ignorada")).toBeVisible();
    expect(putAttempts).toBe(2);
  });

  it("restores eligible totals from the confirmed backend snapshot", async () => {
    const user = userEvent.setup();
    let restored = false;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
      if (init?.method === "PUT") {
        restored = true;
        return transactionJsonResponse({
          transaction_id: transactionId,
          state: "considered",
          changed_at: "2026-07-30T14:00:00Z",
        });
      }
      return transactionJsonResponse(restored ? transactionResult : ignoredTransactionResult);
    });
    renderPage();
    await screen.findByText("Ignorada");
    await user.click(screen.getByRole("button", { name: "Ações de Mercado" }));
    await user.click(await screen.findByRole("menuitem", { name: "Restaurar" }));
    await screen.findByText("Transação restaurada. Totais atualizados.");
    expect(screen.queryByText("Ignorada")).not.toBeInTheDocument();
    expect(screen.getAllByText(/R\$\s*123,45/).length).toBeGreaterThan(1);
  });

  it("shows the remaining reason for a considered but ineligible transaction", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      transactionJsonResponse({
        ...transactionResult,
        items: [
          {
            ...transactionResult.items[0]!,
            provider_status: "CANCELLED",
            inclusion: {
              state: "considered",
              changed_at: "2026-07-30T14:00:00Z",
              origin: "manual",
              rule_name: null,
            },
            totals_eligibility: {
              included: false,
              reason: "ineligible_status",
            },
          },
        ],
        totals: [],
        groups: [{ ...transactionResult.groups[0]!, totals: [] }],
      }),
    );
    renderPage();
    await screen.findByText("Mercado");
    await user.click(screen.getByRole("button", { name: "Ver detalhes de Mercado" }));
    await user.click(screen.getByText("Informações técnicas"));
    expect(screen.getByText("Fora dos totais: situação não elegível")).toBeVisible();
  });
});
