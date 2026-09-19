import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as investmentApi from "../api/investmentPortfolio";
import type { InvestmentAsset } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { InvestmentAssetSettings } from "./InvestmentAssetSettings";

vi.mock("../api/investmentPortfolio");

const asset: InvestmentAsset = {
  id: "33333333-3333-4333-8333-333333333333",
  name: "Petrobras PN",
  ticker: "PETR4",
  asset_type: "Ação",
  currency_code: "BRL",
  created_at: "2026-09-18T12:00:00Z",
  updated_at: "2026-09-18T12:00:00Z",
};

function renderSettings() {
  return render(
    <QueryTestProvider>
      <InvestmentAssetSettings />
    </QueryTestProvider>,
  );
}

describe("InvestmentAssetSettings", () => {
  beforeEach(() => {
    vi.mocked(investmentApi.listInvestmentAssets).mockResolvedValue([]);
  });

  it("lists the investment assets", async () => {
    vi.mocked(investmentApi.listInvestmentAssets).mockResolvedValue([asset]);
    renderSettings();

    expect(await screen.findByText(asset.name)).toBeVisible();
    expect(screen.getByText(asset.ticker!)).toBeVisible();
    expect(screen.getByText(asset.asset_type)).toBeVisible();
  });

  it("creates an asset from settings", async () => {
    const user = userEvent.setup();
    vi.mocked(investmentApi.createInvestmentAsset).mockResolvedValue(asset);
    renderSettings();

    await user.click(await screen.findByRole("button", { name: "Novo ativo" }));
    const drawer = within(screen.getByRole("dialog"));
    await user.type(drawer.getByLabelText("Nome"), asset.name);
    await user.type(drawer.getByLabelText("Código (opcional)"), asset.ticker!);
    await user.type(drawer.getByLabelText("Tipo"), asset.asset_type);
    await user.click(drawer.getByRole("button", { name: "Salvar" }));

    await waitFor(() => expect(investmentApi.createInvestmentAsset).toHaveBeenCalledWith({
      name: asset.name,
      ticker: asset.ticker,
      asset_type: asset.asset_type,
      currency_code: "BRL",
    }));
  });

  it("edits an existing asset", async () => {
    const user = userEvent.setup();
    vi.mocked(investmentApi.listInvestmentAssets).mockResolvedValue([asset]);
    vi.mocked(investmentApi.updateInvestmentAsset).mockResolvedValue({ ...asset, name: "Petrobras" });
    renderSettings();

    await user.click(await screen.findByRole("button", { name: "Editar" }));
    const drawer = within(screen.getByRole("dialog"));
    const name = drawer.getByLabelText("Nome");
    await user.clear(name);
    await user.type(name, "Petrobras");
    await user.click(drawer.getByRole("button", { name: "Salvar" }));

    await waitFor(() => expect(investmentApi.updateInvestmentAsset).toHaveBeenCalledWith(asset.id, {
      name: "Petrobras",
      ticker: asset.ticker,
      asset_type: asset.asset_type,
      currency_code: asset.currency_code,
    }));
  });

  it("deletes an unused asset after confirmation", async () => {
    const user = userEvent.setup();
    vi.mocked(investmentApi.listInvestmentAssets).mockResolvedValue([asset]);
    vi.mocked(investmentApi.deleteInvestmentAsset).mockResolvedValue();
    renderSettings();

    await user.click(await screen.findByRole("button", { name: `Excluir ativo ${asset.name}` }));
    await user.click(await screen.findByRole("button", { name: "Excluir" }));

    await waitFor(() => expect(investmentApi.deleteInvestmentAsset).toHaveBeenCalledWith(asset.id));
  });

  it("validates required fields before saving", async () => {
    const user = userEvent.setup();
    renderSettings();

    await user.click(await screen.findByRole("button", { name: "Novo ativo" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Salvar" }));

    expect(await screen.findByText("Informe o nome do ativo.")).toBeVisible();
    expect(investmentApi.createInvestmentAsset).not.toHaveBeenCalled();
  });
});
