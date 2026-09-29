import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/problems";
import * as statementApi from "../api/statementImports";
import type { ImportPreview } from "../api/statementImports";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { useStatementImports } from "./useStatementImports";

vi.mock("../api/statementImports");

const file = new File(["synthetic"], "sample.csv", { type: "text/csv" });
const counts = { total: 1, new: 1, duplicate: 0, invalid: 0 };
const saved = { run_id: "run", account_id: "acc", counts };

function preview(description: string): ImportPreview {
  return {
    format: "flash_csv", format_version: "1", sha256: description, preview_fingerprint: "fingerprint", filename: "sample.csv",
    period_start: null, period_end: null, currency: "BRL", counts, warnings: [],
    rows: [{
      line_number: 2, occurred_at: null, description, amount: "1.00", currency: "BRL", payment_method: null,
      balance: null, status: "new", errors: [], warnings: [],
    }],
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => { resolve = res; });
  return { promise, resolve };
}

function setup() {
  vi.mocked(statementApi.listImportAccounts).mockResolvedValue([]);
  vi.mocked(statementApi.listStatementImports).mockResolvedValue([]);
  return renderHook(() => useStatementImports(), { wrapper: QueryTestProvider });
}

describe("useStatementImports", () => {
  it("reports each list on its own", async () => {
    vi.mocked(statementApi.listImportAccounts).mockRejectedValue(new ApiError("transport", "offline"));
    vi.mocked(statementApi.listStatementImports).mockResolvedValue([]);
    const { result } = renderHook(() => useStatementImports(), { wrapper: QueryTestProvider });

    expect(result.current.accounts.state.kind).toBe("loading");
    await waitFor(() => expect(result.current.accounts.state.kind).toBe("unavailable"));
    expect(result.current.history.state.kind).toBe("empty");
  });

  it("keeps only the answer to the latest preview request", async () => {
    const older = deferred<ImportPreview>();
    const newer = deferred<ImportPreview>();
    vi.mocked(statementApi.previewStatement).mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise);
    const { result } = setup();

    act(() => { result.current.preview.request(file); });
    act(() => { result.current.preview.request(file, "acc-1"); });
    await act(async () => { newer.resolve(preview("newer")); });
    await waitFor(() => expect(result.current.preview.data?.rows[0].description).toBe("newer"));
    await act(async () => { older.resolve(preview("older")); await new Promise((resolve) => setTimeout(resolve, 20)); });

    expect(result.current.preview.data?.rows[0].description).toBe("newer");
    expect(statementApi.previewStatement).toHaveBeenNthCalledWith(2, file, "acc-1");
  });

  it("forgets a request in flight when the screen discards it", async () => {
    const pending = deferred<ImportPreview>();
    vi.mocked(statementApi.previewStatement).mockReturnValueOnce(pending.promise);
    const { result } = setup();

    act(() => { result.current.preview.request(file); });
    await waitFor(() => expect(result.current.preview.isPending).toBe(true));
    act(() => { result.current.discard(); });
    await waitFor(() => expect(result.current.preview.isPending).toBe(false));
    await act(async () => { pending.resolve(preview("late")); await new Promise((resolve) => setTimeout(resolve, 20)); });

    expect(result.current.preview.data).toBeNull();
  });

  it("refreshes both lists once an import is confirmed, and lets a second submit through only after it settles", async () => {
    const pending = deferred<typeof saved>();
    vi.mocked(statementApi.previewStatement).mockResolvedValue(preview("row"));
    vi.mocked(statementApi.confirmStatement).mockReturnValue(pending.promise);
    const { result } = setup();
    act(() => { result.current.preview.request(file); });
    await waitFor(() => expect(result.current.preview.data).not.toBeNull());
    const request = { file, preview: result.current.preview.data!, target: { newAccountName: "Flash" }, allowPartial: false };

    await act(async () => { result.current.confirm.submit(request); result.current.confirm.submit(request); });
    expect(statementApi.confirmStatement).toHaveBeenCalledTimes(1);
    await act(async () => { pending.resolve(saved); });

    await waitFor(() => expect(result.current.confirm.result).toEqual(saved));
    expect(statementApi.listImportAccounts).toHaveBeenCalledTimes(2);
    expect(statementApi.listStatementImports).toHaveBeenCalledTimes(2);
  });

  it("voids the preview when the server reports a conflict, and a new request clears the conflict", async () => {
    vi.mocked(statementApi.previewStatement).mockResolvedValue(preview("row"));
    vi.mocked(statementApi.confirmStatement).mockRejectedValue(new ApiError("conflict", "Mudou."));
    const { result } = setup();
    act(() => { result.current.preview.request(file); });
    await waitFor(() => expect(result.current.preview.data).not.toBeNull());

    act(() => {
      result.current.confirm.submit({ file, preview: result.current.preview.data!, target: { newAccountName: "Flash" }, allowPartial: false });
    });
    await waitFor(() => expect(result.current.confirm.error?.message).toBe("Mudou."));
    expect(result.current.preview.data).toBeNull();

    act(() => { result.current.preview.request(file); });
    await waitFor(() => expect(result.current.preview.data).not.toBeNull());
    expect(result.current.confirm.error).toBeNull();
  });

  it("downloads the template through the API and releases the lock afterwards", async () => {
    vi.mocked(statementApi.downloadStatementTemplate).mockResolvedValue(undefined);
    const { result } = setup();

    await act(async () => { result.current.template.download(); result.current.template.download(); });
    await waitFor(() => expect(result.current.template.isPending).toBe(false));
    expect(statementApi.downloadStatementTemplate).toHaveBeenCalledTimes(1);

    await act(async () => { result.current.template.download(); });
    expect(statementApi.downloadStatementTemplate).toHaveBeenCalledTimes(2);
  });
});
