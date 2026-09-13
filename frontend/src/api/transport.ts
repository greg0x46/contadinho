// Every API call shares cancellation with the current authenticated browser session.
let generation = 0;
let pending = new AbortController();
const listeners = new Set<() => void>();

export function onSessionExpired(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
export function expireSession() {
  generation++;
  pending.abort();
  pending = new AbortController();
  listeners.forEach((listener) => listener());
}

export async function apiFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
  const started = generation;
  const sessionSignal = pending.signal;
  const headers = new Headers(init.headers);
  const method = (init.method ?? "GET").toUpperCase();
  if (!["GET", "HEAD", "OPTIONS"].includes(method)) headers.set("X-Contadinho-Request", "1");
  const signal = init.signal ? AbortSignal.any([init.signal, sessionSignal]) : sessionSignal;
  const response = await fetch(input, { ...init, headers, signal, credentials: "same-origin" });
  const assertCurrent = () => {
    if (started !== generation || signal.aborted) throw new DOMException("Sessão encerrada", "AbortError");
  };
  assertCurrent();
  const path = String(input).split("?")[0];
  if (response.status === 401 && path !== "/api/auth/login" && path !== "/api/auth/password") {
    expireSession();
    throw new DOMException("Sessão encerrada", "AbortError");
  }
  // Requests that finished receiving headers may still be decoding a large body at logout.
  const json = response.json.bind(response);
  response.json = async () => {
    assertCurrent();
    const value: unknown = await json();
    assertCurrent();
    return value;
  };
  return response;
}
