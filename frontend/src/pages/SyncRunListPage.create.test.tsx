import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/problems";
import { dataSource, runId, syncRun } from "../test/fixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { SyncRunListPage } from "./SyncRunListPage";
import * as dataSourcesApi from "../api/dataSources";
import * as syncRunsApi from "../api/syncRuns";

vi.mock("../api/syncRuns");
vi.mock("../api/dataSources");

const secondRunId = "33333333-3333-4333-8333-333333333333";
const secondSourceId = "44444444-4444-4444-8444-444444444444";

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <Routes>
          <Route path="/" element={<SyncRunListPage />} />
          <Route path="/configuracoes/open-banking/sync-runs/:id" element={<p>Detalhe aberto</p>} />
        </Routes>
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.mocked(syncRunsApi.listSyncRuns).mockResolvedValue([]);
  vi.mocked(dataSourcesApi.listDataSources).mockResolvedValue([dataSource]);
});

describe("sync run creation", () => {
  it("locks synchronously, sends one request and navigates after 202", async () => {
    let resolve!: (value: Awaited<ReturnType<typeof syncRunsApi.createSyncRun>>) => void;
    vi.mocked(syncRunsApi.createSyncRun).mockReturnValue(
      new Promise((done) => {
        resolve = done;
      }),
    );
    renderPage();
    const button = await screen.findByRole("button", { name: "Sincronizar agora" });
    button.click();
    button.click();
    await waitFor(() => expect(syncRunsApi.createSyncRun).toHaveBeenCalledTimes(1));
    resolve({ runs: [syncRun], requested: 1 });
    expect(await screen.findByText("Detalhe aberto")).toBeInTheDocument();
  }, 15_000);

  it("stays put and lists every run when several connections start at once", async () => {
    const second = { ...syncRun, id: secondRunId, source_id: secondSourceId, source_name: "Empresa" };
    vi.mocked(syncRunsApi.createSyncRun).mockResolvedValue({ runs: [syncRun, second], requested: 2 });
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Sincronizar agora" }));

    expect(await screen.findByText("Sincronização iniciada em 2 conexões.")).toBeVisible();
    expect(screen.getByRole("link", { name: /Conta pessoal/ })).toHaveAttribute(
      "href",
      `/configuracoes/open-banking/sync-runs/${runId}`,
    );
    expect(screen.getByRole("link", { name: /Empresa/ })).toHaveAttribute(
      "href",
      `/configuracoes/open-banking/sync-runs/${secondRunId}`,
    );
    expect(screen.queryByText("Detalhe aberto")).not.toBeInTheDocument();
  });

  it("warns when fewer connections start than were requested", async () => {
    vi.mocked(syncRunsApi.createSyncRun).mockResolvedValue({ runs: [syncRun], requested: 2 });
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Sincronizar agora" }));

    expect(
      await screen.findByText("Sincronização iniciada em 1 de 2 conexões."),
    ).toBeVisible();
    expect(
      screen.getByText(/Uma ou mais conexões não iniciaram a sincronização/),
    ).toBeVisible();
    expect(screen.queryByText("Detalhe aberto")).not.toBeInTheDocument();
  });

  it("syncs a single connection from the registry", async () => {
    vi.mocked(syncRunsApi.createSyncRun).mockResolvedValue({ runs: [syncRun], requested: 1 });
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: /Sincronizar$/ }));
    await waitFor(() =>
      expect(syncRunsApi.createSyncRun).toHaveBeenCalledWith(dataSource.id),
    );
  });

  it("shows an active-run link for a confirmed conflict", async () => {
    vi.mocked(syncRunsApi.createSyncRun).mockRejectedValue(
      new ApiError("conflict", "Conflict", {
        type: "/conflict",
        title: "Conflict",
        status: 409,
        active_sync_run_id: runId,
      }),
    );
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Sincronizar agora" }));
    expect(await screen.findByRole("link", { name: "Acompanhar sincronização ativa" })).toHaveAttribute(
      "href",
      `/configuracoes/open-banking/sync-runs/${runId}`,
    );
  });

  // The backend answers the same 409 whether every connection was busy or
  // just the one asked for, so the notice has to know which was asked.
  it("names the single connection in a conflict raised by its own sync button", async () => {
    vi.mocked(syncRunsApi.createSyncRun).mockRejectedValue(
      new ApiError("conflict", "Conflict", {
        type: "/conflict",
        title: "Conflict",
        status: 409,
        active_sync_run_id: runId,
      }),
    );
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: /Sincronizar$/ }));
    expect(await screen.findByText("Essa conexão já está sincronizando.")).toBeVisible();
    expect(screen.queryByText("Todas as conexões já estão sincronizando.")).not.toBeInTheDocument();
  });

  it("communicates uncertainty and permits retry", async () => {
    vi.mocked(syncRunsApi.createSyncRun).mockRejectedValue(new Error("offline"));
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Sincronizar agora" }));
    expect(await screen.findByText(/Não foi possível confirmar/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(screen.getByRole("button", { name: "Sincronizar agora" })).toBeEnabled();
  });
});
