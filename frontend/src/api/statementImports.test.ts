import { afterEach, expect, it, vi } from "vitest";
import { confirmStatement, previewStatement } from "./statementImports";
import { expireSession } from "./transport";

afterEach(() => { vi.unstubAllGlobals(); expireSession(); });

it("accepts omitted fields on an invalid row and sends the preview identity on confirmation", async () => {
  const preview = {
    format: "flash_csv", format_version: "1", sha256: "abc", preview_fingerprint: "fingerprint", filename: "sample.csv",
    currency: "BRL", counts: { total: 1, new: 0, duplicate: 0, invalid: 1 }, warnings: [],
    rows: [{ line_number: 2, status: "invalid", errors: ["valor inválido"], warnings: [] }],
  };
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(new Response(JSON.stringify(preview), { status: 200, headers: { "content-type": "application/json" } }))
    .mockResolvedValueOnce(new Response(JSON.stringify({ run_id: "run", account_id: "acc", counts: preview.counts }),
      { status: 201, headers: { "content-type": "application/json" } }));
  vi.stubGlobal("fetch", fetchMock);
  const file = new File(["Data,Hora\n"], "sample.csv", { type: "text/csv" });

  const parsed = await previewStatement(file);
  expect(parsed.rows[0]).toMatchObject({ occurred_at: null, amount: null, description: "", errors: ["valor inválido"] });
  await confirmStatement(file, parsed, { newAccountName: "Flash" }, true);
  const init = fetchMock.mock.calls[1][1] as RequestInit;
  const body = init.body as FormData;
  expect(body.get("expected_sha256")).toBe("abc");
  expect(body.get("expected_preview_fingerprint")).toBe("fingerprint");
  expect(body.get("expected_format_version")).toBe("1");
  expect(body.get("new_account_name")).toBe("Flash");
  expect(body.get("allow_partial")).toBe("true");
  expect(new Headers(init.headers).get("X-Contadinho-Request")).toBe("1");
  expect(new Headers(init.headers).has("content-type")).toBe(false);
});
