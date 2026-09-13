import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, type ReactNode } from "react";
import { getSession } from "../api/auth";
import { expireSession, onSessionExpired } from "../api/transport";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { LoginPage } from "../pages/LoginPage";

export function AuthGate({ children }: { children: ReactNode }) {
  const client = useQueryClient();
  const wasAuthenticated = useRef(false);
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["auth-session"], queryFn: ({ signal }) => getSession(signal),
    retry: false, refetchInterval: 60_000,
  });
  useEffect(() => onSessionExpired(() => {
    void client.cancelQueries();
    wasAuthenticated.current = false;
    client.removeQueries({ predicate: (query) => query.queryKey[0] !== "auth-session" });
    client.getMutationCache().clear();
    client.setQueryData(["auth-session"], { authenticated: false });
  }), [client]);
  // Another browser/tab can revoke the cookie's server-side session.
  useEffect(() => {
    if (data?.authenticated === false && wasAuthenticated.current) {
      wasAuthenticated.current = false;
      expireSession();
    }
    if (data?.authenticated === true) wasAuthenticated.current = true;
    if (data?.authenticated === false) {
      void client.cancelQueries({ predicate: (query) => query.queryKey[0] !== "auth-session" });
      client.removeQueries({ predicate: (query) => query.queryKey[0] !== "auth-session" });
    }
  }, [client, data?.authenticated]);
  if (isLoading) return <LoadingState>Verificando sessão…</LoadingState>;
  if (error || !data) return <UnavailableState onRetry={() => refetch()}>Não foi possível verificar sua sessão.</UnavailableState>;
  if (!data.authenticated) return <LoginPage onDone={() => { void client.invalidateQueries({ queryKey: ["auth-session"] }); }} />;
  return <>{children}</>;
}
