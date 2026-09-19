import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { dataSource, syncRun } from "../test/fixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { SyncRunListPage } from "./SyncRunListPage";
import * as dataSourcesApi from "../api/dataSources";
import * as syncRunsApi from "../api/syncRuns";

vi.mock("../api/syncRuns");
vi.mock("../api/dataSources");

const renderPage = () =>
  render(
    <MemoryRouter>
      <QueryTestProvider>
        <SyncRunListPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );

beforeEach(() => {
  vi.mocked(syncRunsApi.createSyncRun).mockReset();
  vi.mocked(dataSourcesApi.listDataSources).mockResolvedValue([dataSource]);
});

describe("recent history", () => {
  it("distinguishes a confirmed empty history", async () => {
    vi.mocked(syncRunsApi.listSyncRuns).mockResolvedValue([]);
    renderPage();
    expect(await screen.findByText("Nenhuma sincronização ainda")).toBeVisible();
  });

  it("shows unavailability and retries", async () => {
    vi.mocked(syncRunsApi.listSyncRuns).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce([]);
    renderPage();
    expect(await screen.findByText(/temporariamente indisponível/)).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(await screen.findByText("Nenhuma sincronização ainda")).toBeVisible();
  });

  it("renders received runs, metrics and accessible detail links", async () => {
    vi.mocked(syncRunsApi.listSyncRuns).mockResolvedValue([syncRun]);
    renderPage();
    expect(await screen.findByText("Em andamento")).toBeVisible();
    expect(screen.getByRole("columnheader", { name: "Contas processadas" })).toBeVisible();
    expect(screen.getByRole("link", { name: /Ver detalhes/ })).toHaveAttribute(
      "href",
      `/configuracoes/open-banking/sync-runs/${syncRun.id}`,
    );
  });
});
