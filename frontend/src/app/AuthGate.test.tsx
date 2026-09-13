import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { AuthGate } from "./AuthGate";
import { expireSession } from "../api/transport";
import { AuthenticationSettings } from "../components/AuthenticationSettings";

const json = (value: unknown) => new Response(JSON.stringify(value), { headers: { "Content-Type": "application/json" } });
afterEach(() => vi.unstubAllGlobals());
function page() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><AuthGate><p>Dados financeiros privados</p><AuthenticationSettings /></AuthGate></QueryClientProvider>);
  return client;
}
it("requires login, shows private content and clears it on logout", async () => {
  let authenticated = false;
  vi.stubGlobal("fetch", vi.fn(async (path: string) => {
    if (path === "/api/auth/login") authenticated = true;
    if (path === "/api/auth/logout") authenticated = false;
    return json({ authenticated });
  }));
  const client = page();
  await screen.findByRole("button", { name: "Entrar" });
  expect(screen.queryByText("Dados financeiros privados")).not.toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("E-mail"), { target: { value: "owner@example.com" } });
  fireEvent.change(screen.getByLabelText("Senha"), { target: { value: "correct horse battery staple" } });
  fireEvent.click(screen.getByRole("button", { name: "Entrar" }));
  await screen.findByText("Dados financeiros privados");
  client.setQueryData(["accounts"], [{ balance: "private" }]);
  fireEvent.click(screen.getByRole("button", { name: "Sair" }));
  await screen.findByRole("button", { name: "Entrar" });
  expect(screen.queryByText("Dados financeiros privados")).not.toBeInTheDocument();
  expect(client.getQueryData(["accounts"])).toBeUndefined();
});
it("clears cached private data when the session expires", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => json({ authenticated: true })));
  const client = page();
  await screen.findByText("Dados financeiros privados");
  client.setQueryData(["accounts"], ["private"]);
  act(() => expireSession());
  await screen.findByRole("button", { name: "Entrar" });
  await waitFor(() => expect(client.getQueryData(["accounts"])).toBeUndefined());
});
it("changes the password and returns to login", async () => {
  const mock = vi.fn(async (path: string) => json({ authenticated: path !== "/api/auth/password" }));
  vi.stubGlobal("fetch", mock); page();
  await screen.findByText("Dados financeiros privados");
  fireEvent.change(screen.getByLabelText("Senha atual"), { target: { value: "correct horse battery staple" } });
  fireEvent.change(screen.getByLabelText("Nova senha (15–128 caracteres)"), { target: { value: "a completely different password" } });
  fireEvent.change(screen.getByLabelText("Repita a nova senha"), { target: { value: "a completely different password" } });
  fireEvent.click(screen.getByRole("button", { name: "Alterar senha e encerrar sessões" }));
  await screen.findByRole("button", { name: "Entrar" });
  expect(mock).toHaveBeenCalledWith("/api/auth/password", expect.objectContaining({ method: "PUT" }));
});
