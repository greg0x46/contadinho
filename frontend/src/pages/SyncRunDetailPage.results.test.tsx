import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/problems";
import { runId, syncRunDetail } from "../test/fixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { SyncRunDetailPage } from "./SyncRunDetailPage";
import * as syncRunsApi from "../api/syncRuns";

vi.mock("../api/syncRuns");

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={[`/open-banking/sync-runs/${runId}`]}>
      <QueryTestProvider>
        <Routes>
          <Route path="/open-banking/sync-runs/:id" element={<SyncRunDetailPage />} />
        </Routes>
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("detail results", () => {
  it.each([
    ["in_progress", "Em andamento"],
    ["completed", "Concluída"],
    ["completed_with_failures", "Concluída com falhas"],
    ["failed", "Falhou"],
  ] as const)("renders %s as %s with complete metrics", async (status, label) => {
    vi.mocked(syncRunsApi.getSyncRun).mockResolvedValue({
      ...syncRunDetail,
      status,
      finished_at: status === "in_progress" ? null : "2026-07-29T12:01:00Z",
    });
    renderDetail();
    expect(await screen.findByText(label)).toBeVisible();
    expect(screen.getByText("Transações atualizadas")).toBeVisible();
  });

  it("distinguishes a valid missing UUID", async () => {
    vi.mocked(syncRunsApi.getSyncRun).mockRejectedValue(new ApiError("not_found", "missing"));
    renderDetail();
    expect(await screen.findByText("Sincronização não encontrada")).toBeVisible();
  });
});
