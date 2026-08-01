import { afterEach, describe, expect, it, vi } from "vitest";

import { requestJson } from "./client";
import { ApiError } from "./problems";
import { jsonResponse } from "../test/fixtures";

afterEach(() => vi.unstubAllGlobals());

describe("requestJson", () => {
  it("validates status, content type and payload", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ ok: true })));
    await expect(requestJson("/", {}, (value) => value, 200)).resolves.toEqual({ ok: true });
  });

  it("rejects wrong content types and malformed JSON", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("<html>", { status: 200, headers: { "content-type": "text/html" } })),
    );
    await expect(requestJson("/", {}, (value) => value, 200)).rejects.toMatchObject({
      kind: "response",
    });
  });

  it("classifies transport errors and propagates aborts", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("secret transport detail")));
    await expect(requestJson("/", {}, (value) => value, 200)).rejects.toMatchObject({
      kind: "transport",
    } satisfies Partial<ApiError>);

    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new DOMException("Aborted", "AbortError")));
    await expect(requestJson("/", {}, (value) => value, 200)).rejects.toMatchObject({
      name: "AbortError",
    });
  });

  it("parses application/problem+json conflicts", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(
          { type: "/conflict", title: "Conflict", status: 409 },
          { status: 409, headers: { "content-type": "application/problem+json" } },
        ),
      ),
    );
    await expect(requestJson("/", {}, (value) => value, 202)).rejects.toMatchObject({
      kind: "conflict",
    });
  });
});
