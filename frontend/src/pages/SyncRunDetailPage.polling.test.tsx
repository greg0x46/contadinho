import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { runId, syncRunDetail } from "../test/fixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { SyncRunDetailPage } from "./SyncRunDetailPage";
import * as syncRunsApi from "../api/syncRuns";

vi.mock("../api/syncRuns");

function renderDetail(id: string) {
  return render(
    <MemoryRouter initialEntries={[`/open-banking/sync-runs/${id}`]}>
      <QueryTestProvider>
        <Routes>
          <Route path="/open-banking/sync-runs/:id" element={<SyncRunDetailPage />} />
        </Routes>
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.mocked(syncRunsApi.getSyncRun).mockReset();
});

describe("detail polling states", () => {
  it("rejects an invalid UUID without requesting the API", () => {
    renderDetail("not-a-uuid");
    expect(screen.getByRole("heading", { name: "Endereço de sincronização inválido" })).toBeVisible();
    expect(syncRunsApi.getSyncRun).not.toHaveBeenCalled();
  });

  it("shows loading while the API response is pending", async () => {
    let resolve!: (value: typeof syncRunDetail) => void;
    vi.mocked(syncRunsApi.getSyncRun).mockReturnValue(
      new Promise((done) => {
        resolve = done;
      }),
    );
    renderDetail(runId);
    expect(screen.getByText("Carregando sincronização…")).toBeVisible();
    resolve({ ...syncRunDetail, status: "completed", finished_at: "2026-07-29T12:01:00Z" });
    expect(await screen.findByText("Concluída")).toBeVisible();
  });

  it("renders a final confirmed snapshot", async () => {
    vi.mocked(syncRunsApi.getSyncRun).mockResolvedValue({
      ...syncRunDetail,
      status: "completed",
      finished_at: "2026-07-29T12:01:00Z",
    });
    renderDetail(runId);
    expect(await screen.findByText("Concluída")).toBeVisible();
  });
});
