import { afterEach, describe, expect, it, vi } from "vitest";

import { listSyncRuns } from "./syncRuns";
import { jsonResponse, syncRun } from "../test/fixtures";

afterEach(() => vi.unstubAllGlobals());

describe("listSyncRuns", () => {
  it("requests the fixed limit and validates the received order", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse([syncRun]));
    vi.stubGlobal("fetch", fetchMock);
    await expect(listSyncRuns()).resolves.toEqual([syncRun]);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/sync-runs?limit=20",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("rejects unavailable and malformed responses", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ runs: [] })));
    await expect(listSyncRuns()).rejects.toMatchObject({ kind: "response" });
  });
});
