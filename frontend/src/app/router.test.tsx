import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { AppRouter } from "./router";

vi.mock("./AuthGate", () => ({ AuthGate: ({ children }: { children: ReactNode }) => children }));
vi.mock("../pages/GeneralSettingsPage", () => ({ GeneralSettingsPage: () => <h1>Seção geral</h1> }));
vi.mock("../pages/ScenariosPage", () => ({ ScenariosPage: () => <h1>Seção cenários</h1> }));
vi.mock("../pages/AutomationRulesPage", () => ({ AutomationRulesPage: () => <h1>Seção automações</h1> }));
vi.mock("../pages/CategoriesPage", () => ({ CategoriesPage: () => <h1>Seção categorias</h1> }));
vi.mock("../pages/SyncRunListPage", () => ({ SyncRunListPage: () => <h1>Seção Open Banking</h1> }));
vi.mock("../pages/SyncRunDetailPage", () => ({ SyncRunDetailPage: () => <h1>Detalhe da sincronização</h1> }));
vi.mock("../pages/InvestmentAssetsSettingsPage", () => ({ InvestmentAssetsSettingsPage: () => <h1>Seção ativos</h1> }));

describe("settings routes", () => {
  it.each([
    ["geral", "Seção geral"], ["cenarios", "Seção cenários"],
    ["automacoes", "Seção automações"], ["categorias", "Seção categorias"],
    ["open-banking", "Seção Open Banking"], ["ativos-de-investimento", "Seção ativos"],
    ["open-banking/sync-runs/123", "Detalhe da sincronização"],
  ])("opens %s directly with the settings menu selected", async (path, heading) => {
    window.history.replaceState({}, "", `/configuracoes/${path}`);
    render(<AppRouter />);
    expect(await screen.findByRole("heading", { name: heading })).toBeVisible();
    expect(screen.getByRole("menuitem", { name: /Configurações/ })).toHaveClass("ant-menu-item-selected");
    for (const name of ["Cenários", "Automações", "Categorias", "Open Banking"]) {
      expect(screen.queryByRole("menuitem", { name: new RegExp(name) })).not.toBeInTheDocument();
    }
  });

  it("opens security directly", async () => {
    window.history.replaceState({}, "", "/configuracoes/seguranca");
    render(<AppRouter />);
    expect(await screen.findByLabelText("Senha atual")).toBeVisible();
  });

  it.each(["cenarios", "automacoes", "categorias", "open-banking", "open-banking/sync-runs/123"])("redirects the registered legacy route %s", async (path) => {
    window.history.replaceState({}, "", `/${path}?page=2#detalhes`);
    render(<AppRouter />);
    await screen.findByRole("heading", { name: /Seção|Detalhe da sincronização/ });
    expect(window.location.pathname + window.location.search + window.location.hash).toBe(`/configuracoes/${path}?page=2#detalhes`);
  });
});
