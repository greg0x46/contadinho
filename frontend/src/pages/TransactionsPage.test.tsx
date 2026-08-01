import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import { TransactionsPage } from "./TransactionsPage";
import { QueryTestProvider } from "../test/QueryTestProvider";
import {
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

  it("renders compact BRL totals, translated facts and query state", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(transactionJsonResponse());
    renderPage();
    expect(await screen.findByRole("heading", { name: "Transações" })).toBeVisible();
    expect(await screen.findByText("Mercado", {}, { timeout: 10_000 })).toBeVisible();
    expect(screen.getByText("Confirmada")).toBeVisible();
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

  it("restores filters and grouping from the URL", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(transactionJsonResponse());
    renderPage("/transacoes?description=Mercado&provider_status=PENDING&group=day");
    await screen.findByText("Mercado");
    const request = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(request.group_by).toBe("day");
    expect(request.filters).toMatchObject({
      description: "Mercado",
      provider_status: "PENDING",
    });
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
    await user.click(screen.getByRole("button", { name: "Ignorar Mercado" }));
    expect(
      await screen.findByText("Transação ignorada. Totais atualizados."),
    ).toBeInTheDocument();
    expect(screen.getByText("Ignorada")).toBeVisible();
    expect(screen.getByText("1 transação")).toBeVisible();
    expect(screen.getByText(/permanecem na lista/)).toBeVisible();
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
    await user.click(screen.getByRole("button", { name: "Ignorar Mercado" }));
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
    await user.click(screen.getByRole("button", { name: "Restaurar Mercado" }));
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
