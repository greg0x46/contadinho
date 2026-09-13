import { afterEach, expect, it, vi } from "vitest";
import { apiFetch, expireSession, onSessionExpired } from "./transport";

afterEach(() => { vi.unstubAllGlobals(); expireSession(); });
it("includes cookies and CSRF header and respects caller cancellation", async () => {
  const mock = vi.fn().mockResolvedValue(new Response("{}")); vi.stubGlobal("fetch", mock);
  const controller = new AbortController();
  await apiFetch("/api/preferences", { method: "PUT", signal: controller.signal });
  const init = mock.mock.calls[0][1] as RequestInit;
  expect(init.credentials).toBe("same-origin");
  expect(new Headers(init.headers).get("X-Contadinho-Request")).toBe("1");
  controller.abort(); expect(init.signal?.aborted).toBe(true);
});
it("rejects a late response after logout even when fetch ignores cancellation", async () => {
  let resolve!: (value: Response) => void;
  vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((done) => { resolve = done; })));
  const request = apiFetch("/api/accounts");
  const rejected = expect(request).rejects.toMatchObject({ name: "AbortError" });
  expireSession(); resolve(new Response('{"private":true}'));
  await rejected;
});
it("rejects a body whose headers arrived before logout", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response('{"private":true}')));
  const response = await apiFetch("/api/accounts");
  expireSession();
  await expect(response.json()).rejects.toMatchObject({ name: "AbortError" });
});
it("expires on an API 401 but preserves the login error response", async () => {
  const listener = vi.fn(); const stop = onSessionExpired(listener);
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(new Response("{}", { status: 401 }))));
  await expect(apiFetch("/api/accounts")).rejects.toMatchObject({ name: "AbortError" });
  expect(listener).toHaveBeenCalledOnce();
  expect((await apiFetch("/api/auth/login", { method: "POST" })).status).toBe(401);
  expect(listener).toHaveBeenCalledOnce(); stop();
});
