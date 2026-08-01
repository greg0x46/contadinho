import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/problems";
import { runId, syncRun } from "../test/fixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { SyncRunListPage } from "./SyncRunListPage";
import * as syncRunsApi from "../api/syncRuns";

vi.mock("../api/syncRuns");

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <Routes>
          <Route path="/" element={<SyncRunListPage />} />
          <Route path="/open-banking/sync-runs/:id" element={<p>Detalhe aberto</p>} />
        </Routes>
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.mocked(syncRunsApi.listSyncRuns).mockResolvedValue([]);
});

describe("sync run creation", () => {
  it("locks synchronously, sends one request and navigates after 202", async () => {
    let resolve!: (value: typeof syncRun) => void;
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
    resolve(syncRun);
    expect(await screen.findByText("Detalhe aberto")).toBeInTheDocument();
  }, 15_000);

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
      `/open-banking/sync-runs/${runId}`,
    );
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
