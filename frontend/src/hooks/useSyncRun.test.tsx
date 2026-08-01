import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/problems";
import { runId, syncRunDetail } from "../test/fixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { useSyncRun } from "./useSyncRun";
import * as syncRunsApi from "../api/syncRuns";

vi.mock("../api/syncRuns");

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useSyncRun", () => {
  it("loads immediately, polls after three seconds and absorbs a final state", async () => {
    vi.mocked(syncRunsApi.getSyncRun)
      .mockResolvedValueOnce(syncRunDetail)
      .mockResolvedValueOnce({
        ...syncRunDetail,
        status: "completed",
        finished_at: "2026-07-29T12:01:00Z",
      });
    const { result } = renderHook(() => useSyncRun(runId), { wrapper: QueryTestProvider });
    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));
    expect(syncRunsApi.getSyncRun).toHaveBeenCalledTimes(1);
    await act(() => vi.advanceTimersByTimeAsync(3000));
    await waitFor(() => expect(result.current.state.snapshot?.status).toBe("completed"));
    await act(() => vi.advanceTimersByTimeAsync(9000));
    expect(syncRunsApi.getSyncRun).toHaveBeenCalledTimes(2);
  });

  it("preserves a confirmed snapshot when a poll becomes unavailable", async () => {
    vi.mocked(syncRunsApi.getSyncRun)
      .mockResolvedValueOnce(syncRunDetail)
      .mockRejectedValueOnce(new ApiError("transport", "offline"));
    const { result } = renderHook(() => useSyncRun(runId), { wrapper: QueryTestProvider });
    await waitFor(() => expect(result.current.state.snapshot).not.toBeNull());
    await act(() => vi.advanceTimersByTimeAsync(3000));
    await waitFor(() => expect(result.current.state.freshness).toBe("stale"));
    expect(result.current.state.snapshot).toEqual(syncRunDetail);
  });

  it("reports unavailability without inventing a snapshot", async () => {
    vi.mocked(syncRunsApi.getSyncRun).mockRejectedValue(new ApiError("transport", "offline"));
    const { result } = renderHook(() => useSyncRun(runId), { wrapper: QueryTestProvider });
    await waitFor(() => expect(result.current.state.freshness).toBe("unavailable"));
    expect(result.current.state.snapshot).toBeNull();
  });

  it("preserves a stale snapshot during a manual retry", async () => {
    let resolveRetry!: (value: typeof syncRunDetail) => void;
    vi.mocked(syncRunsApi.getSyncRun)
      .mockResolvedValueOnce(syncRunDetail)
      .mockRejectedValueOnce(new ApiError("transport", "offline"))
      .mockReturnValueOnce(
        new Promise((resolve) => {
          resolveRetry = resolve;
        }),
      );
    const { result } = renderHook(() => useSyncRun(runId), { wrapper: QueryTestProvider });
    await waitFor(() => expect(result.current.state.snapshot).toEqual(syncRunDetail));
    await act(() => vi.advanceTimersByTimeAsync(3000));
    await waitFor(() => expect(result.current.state.freshness).toBe("stale"));
    act(() => result.current.retry());
    expect(result.current.state.snapshot).toEqual(syncRunDetail);
    await waitFor(() => expect(result.current.state.retrying).toBe(true));
    resolveRetry({ ...syncRunDetail, status: "completed", finished_at: "2026-07-29T12:01:00Z" });
    await waitFor(() => expect(result.current.state.freshness).toBe("fresh"));
  });

  it("aborts on unmount", async () => {
    vi.mocked(syncRunsApi.getSyncRun).mockImplementation(
      (_id, signal) =>
        new Promise((_resolve, reject) => {
          signal?.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")));
        }),
    );
    const { unmount } = renderHook(() => useSyncRun(runId), { wrapper: QueryTestProvider });
    unmount();
    expect(vi.mocked(syncRunsApi.getSyncRun).mock.calls[0]?.[1]?.aborted).toBe(true);
  });
});
